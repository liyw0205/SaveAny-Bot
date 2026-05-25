package transfer

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/krau/SaveAny-Bot/core"
	"github.com/krau/SaveAny-Bot/pkg/enums/tasktype"
	"github.com/krau/SaveAny-Bot/pkg/storagetypes"
	"github.com/krau/SaveAny-Bot/storage"
	"github.com/rs/xid"
)

var _ core.Executable = (*Task)(nil)

type TaskElement struct {
	ID            string
	SourceStorage storage.Storage
	SourcePath    string
	FileInfo      storagetypes.FileInfo
	TargetStorage storage.Storage
	TargetPath    string
}

type Task struct {
	ID           string
	ctx          context.Context
	elems        []TaskElement
	Progress     ProgressTracker
	IgnoreErrors bool
	downloaded   atomic.Int64
	uploaded     atomic.Int64
	totalSize    int64
	processing   map[string]TaskElementInfo
	processingMu sync.RWMutex
	failed       map[string]error
	targetPathMu sync.RWMutex
	targetPath   string
	pathVersion  atomic.Int64
	phaseMu      sync.RWMutex
	phase        string
	uploadMu     sync.RWMutex
	uploading    map[string]int64
	uploadCancel map[string]context.CancelFunc
}

// Title implements core.Executable.
func (t *Task) Title() string {
	return fmt.Sprintf("[%s](%d files/%.2fMB)", t.Type(), len(t.elems), float64(t.totalSize)/(1024*1024))
}

// Type implements core.Executable.
func (t *Task) Type() tasktype.TaskType {
	return tasktype.TaskTypeTransfer
}

// TaskID implements core.Executable.
func (t *Task) TaskID() string {
	return t.ID
}

func NewTaskElement(
	sourceStorage storage.Storage,
	fileInfo storagetypes.FileInfo,
	targetStorage storage.Storage,
	targetPath string,
) *TaskElement {
	id := xid.New().String()
	return &TaskElement{
		ID:            id,
		SourceStorage: sourceStorage,
		SourcePath:    fileInfo.Path,
		FileInfo:      fileInfo,
		TargetStorage: targetStorage,
		TargetPath:    targetPath,
	}
}

func NewTransferTask(
	id string,
	ctx context.Context,
	elems []TaskElement,
	progress ProgressTracker,
	ignoreErrors bool,
) *Task {
	task := &Task{
		ID:       id,
		ctx:      ctx,
		elems:    elems,
		Progress: progress,
		phase:    "queued",
		uploaded: atomic.Int64{},
		targetPath: func() string {
			if len(elems) == 0 {
				return ""
			}
			return elems[0].TargetPath
		}(),
		totalSize: func() int64 {
			var total int64
			for _, elem := range elems {
				total += elem.FileInfo.Size
			}
			return total
		}(),
		processing:   make(map[string]TaskElementInfo),
		IgnoreErrors: ignoreErrors,
		failed:       make(map[string]error),
		uploading:    make(map[string]int64),
		uploadCancel: make(map[string]context.CancelFunc),
	}
	return task
}

func (t *Task) UpdateTargetPath(targetPath string) {
	t.targetPathMu.Lock()
	t.targetPath = targetPath
	t.targetPathMu.Unlock()
	t.pathVersion.Add(1)
	t.uploaded.Store(0)

	t.uploadMu.Lock()
	t.uploading = make(map[string]int64)
	for _, cancel := range t.uploadCancel {
		cancel()
	}
	t.uploadMu.Unlock()
}

func (t *Task) currentTargetPath() string {
	t.targetPathMu.RLock()
	defer t.targetPathMu.RUnlock()
	return t.targetPath
}

func (t *Task) setPhase(phase string) {
	t.phaseMu.Lock()
	t.phase = phase
	t.phaseMu.Unlock()
}

func (t *Task) registerUploadCancel(id string, cancel context.CancelFunc) {
	t.uploadMu.Lock()
	defer t.uploadMu.Unlock()
	t.uploadCancel[id] = cancel
}

func (t *Task) clearUploadCancel(id string) {
	t.uploadMu.Lock()
	defer t.uploadMu.Unlock()
	delete(t.uploadCancel, id)
	delete(t.uploading, id)
}

func (t *Task) setUploadingBytes(id string, n int64) {
	t.uploadMu.Lock()
	defer t.uploadMu.Unlock()
	t.uploading[id] = n
}

func (t *Task) uploadedInFlight() int64 {
	t.uploadMu.RLock()
	defer t.uploadMu.RUnlock()
	var total int64
	for _, n := range t.uploading {
		total += n
	}
	return total
}
