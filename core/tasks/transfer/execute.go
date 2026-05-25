package transfer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/charmbracelet/log"
	"github.com/krau/SaveAny-Bot/common/utils/ioutil"
	"github.com/krau/SaveAny-Bot/config"
	"github.com/krau/SaveAny-Bot/pkg/enums/ctxkey"
	"github.com/krau/SaveAny-Bot/storage"
	"golang.org/x/sync/errgroup"
)

var errTargetPathChanged = errors.New("target path changed")

// Execute implements core.Executable.
func (t *Task) Execute(ctx context.Context) error {
	logger := log.FromContext(ctx).WithPrefix(fmt.Sprintf("transfer[%s]", t.ID))
	logger.Info("Starting transfer task")
	t.setPhase("running")
	if t.Progress != nil {
		t.Progress.OnStart(ctx, t)
	}

	workers := config.C().Workers
	eg, gctx := errgroup.WithContext(ctx)
	eg.SetLimit(workers)

	for _, elem := range t.elems {
		eg.Go(func() error {
			t.processingMu.RLock()
			if t.processing[elem.ID] != nil {
				t.processingMu.RUnlock()
				return fmt.Errorf("element with ID %s is already being processed", elem.ID)
			}
			t.processingMu.RUnlock()

			t.processingMu.Lock()
			t.processing[elem.ID] = &elem
			t.processingMu.Unlock()

			defer func() {
				t.processingMu.Lock()
				delete(t.processing, elem.ID)
				t.processingMu.Unlock()
			}()

			err := t.processElement(gctx, elem)
			if err != nil && !t.IgnoreErrors {
				return err
			}
			if err != nil {
				t.processingMu.Lock()
				t.failed[elem.ID] = err
				t.processingMu.Unlock()
				logger.Errorf("Failed to process file %s: %v", elem.FileInfo.Name, err)
			}
			return nil
		})
	}

	err := eg.Wait()
	if err != nil {
		logger.Errorf("Error during transfer processing: %v", err)
	} else {
		logger.Info("Transfer task completed successfully")
	}

	if t.Progress != nil {
		t.Progress.OnDone(ctx, t, err)
	}
	return err
}

func (t *Task) processElement(ctx context.Context, elem TaskElement) error {
	logger := log.FromContext(ctx).WithPrefix(fmt.Sprintf("file[%s]", elem.FileInfo.Name))
	uploadedSize := elem.FileInfo.Size

	// Check whether the source storage supports reading
	readableStorage, ok := elem.SourceStorage.(storage.StorageReadable)
	if !ok {
		return fmt.Errorf("source storage %s does not support reading", elem.SourceStorage.Name())
	}

	if config.C().Stream {
		for {
			logger.Info("Opening file from source storage")
			reader, size, err := readableStorage.OpenFile(ctx, elem.SourcePath)
			if err != nil {
				return fmt.Errorf("failed to open file: %w", err)
			}
			uploadedSize = size
			err = t.uploadReader(ctx, elem, reader, size)
			closeErr := reader.Close()
			if err == nil && closeErr != nil {
				err = closeErr
			}
			if errors.Is(err, errTargetPathChanged) {
				continue
			}
			if err != nil {
				return fmt.Errorf("failed to upload file to storage: %w", err)
			}
			break
		}
	} else {
		logger.Info("Opening file from source storage")
		reader, size, err := readableStorage.OpenFile(ctx, elem.SourcePath)
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		uploadedSize = size
		defer reader.Close()

		logger.Info("Downloading to temporary file for ReadSeeker support")
		tempFile, err := t.downloadToTemp(ctx, reader, elem.FileInfo.Name)
		if err != nil {
			return fmt.Errorf("failed to download to temp: %w", err)
		}
		defer os.Remove(tempFile.Name())
		defer tempFile.Close()

		if _, err := tempFile.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("failed to seek temp file: %w", err)
		}

		logger.Infof("Uploading file to storage (size: %d bytes)", size)
		for {
			if _, err := tempFile.Seek(0, io.SeekStart); err != nil {
				return fmt.Errorf("failed to seek temp file: %w", err)
			}
			if err := t.uploadReader(ctx, elem, tempFile, size); err != nil {
				if errors.Is(err, errTargetPathChanged) {
					continue
				}
				return fmt.Errorf("failed to upload file to storage: %w", err)
			}
			break
		}
	}

	t.uploaded.Add(uploadedSize)
	if t.Progress != nil {
		t.Progress.OnProgress(ctx, t)
	}

	logger.Info("File uploaded successfully")
	return nil
}

func (t *Task) downloadToTemp(ctx context.Context, reader io.Reader, filename string) (*os.File, error) {
	tempDir := config.C().Temp.BasePath
	if tempDir == "" {
		tempDir = os.TempDir()
	}

	tempFile, err := os.CreateTemp(tempDir, filepath.Base(filename)+"-*.tmp")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	t.setPhase("downloading")
	if t.Progress != nil {
		t.Progress.OnProgress(ctx, t)
	}
	wr := ioutil.NewProgressWriter(tempFile, func(n int) {
		t.downloaded.Add(int64(n))
		if t.Progress != nil {
			t.Progress.OnProgress(ctx, t)
		}
	})

	if _, err := io.Copy(wr, reader); err != nil {
		tempFile.Close()
		os.Remove(tempFile.Name())
		return nil, fmt.Errorf("failed to copy to temp file: %w", err)
	}

	return tempFile, nil
}

func (t *Task) uploadReader(ctx context.Context, elem TaskElement, reader io.Reader, size int64) error {
	version := t.pathVersion.Load()
	uploadCtx, cancel := context.WithCancel(ctx)
	t.registerUploadCancel(elem.ID, cancel)
	defer func() {
		cancel()
		t.clearUploadCancel(elem.ID)
	}()

	t.setPhase("uploading")
	if t.Progress != nil {
		t.Progress.OnProgress(uploadCtx, t)
	}
	storagePath := path.Join(t.currentTargetPath(), elem.FileInfo.Name)
	uploadCtx = context.WithValue(uploadCtx, ctxkey.ContentLength, size)
	progressReader := ioutil.NewProgressReader(asReadSeeker(reader), size, func(read int64, total int64) {
		t.setUploadingBytes(elem.ID, read)
		if t.Progress != nil {
			t.Progress.OnProgress(uploadCtx, t)
		}
	})
	if err := elem.TargetStorage.Save(uploadCtx, progressReader, storagePath); err != nil {
		if uploadCtx.Err() != nil && ctx.Err() == nil && version != t.pathVersion.Load() {
			return errTargetPathChanged
		}
		return err
	}
	return nil
}

func asReadSeeker(reader io.Reader) io.ReadSeeker {
	if rs, ok := reader.(io.ReadSeeker); ok {
		return rs
	}
	return readSeekerAdapter{Reader: reader}
}

type readSeekerAdapter struct {
	io.Reader
}

func (r readSeekerAdapter) Seek(offset int64, whence int) (int64, error) {
	if offset == 0 && whence == io.SeekStart {
		return 0, nil
	}
	return 0, fmt.Errorf("reader is not seekable")
}
