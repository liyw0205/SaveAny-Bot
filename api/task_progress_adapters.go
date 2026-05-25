package api

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/krau/SaveAny-Bot/core/tasks/aria2dl"
	"github.com/krau/SaveAny-Bot/core/tasks/batchtfile"
	"github.com/krau/SaveAny-Bot/core/tasks/directlinks"
	"github.com/krau/SaveAny-Bot/core/tasks/parsed"
	tphtask "github.com/krau/SaveAny-Bot/core/tasks/telegraph"
	"github.com/krau/SaveAny-Bot/core/tasks/tfile"
	"github.com/krau/SaveAny-Bot/core/tasks/transfer"
	"github.com/krau/SaveAny-Bot/core/tasks/ytdlp"
	"github.com/krau/SaveAny-Bot/pkg/aria2"
)

type directLinksAPIProgress struct{ taskID string }

func newDirectLinksAPIProgress(taskID string) directlinks.ProgressTracker {
	return &directLinksAPIProgress{taskID: taskID}
}

func (p *directLinksAPIProgress) OnStart(ctx context.Context, info directlinks.TaskInfo) {
	if task, ok := GetTask(p.taskID); ok {
		task.UpdateStatus(TaskStatusRunning)
		task.UpdatePhase("downloading")
		task.TotalFiles = info.TotalFiles()
	}
}

func (p *directLinksAPIProgress) OnProgress(ctx context.Context, info directlinks.TaskInfo) {
	if task, ok := GetTask(p.taskID); ok {
		task.UpdatePhase("downloading")
		task.UpdateDownloadProgress(info.DownloadedBytes(), info.TotalBytes())
		task.TotalFiles = info.TotalFiles()
	}
}

func (p *directLinksAPIProgress) OnDone(ctx context.Context, info directlinks.TaskInfo, err error) {
	finishAPITask(p.taskID, err)
}

type ytdlpAPIProgress struct{ taskID string }

func newYTDLPAPIProgress(taskID string) ytdlp.ProgressTracker {
	return &ytdlpAPIProgress{taskID: taskID}
}

func (p *ytdlpAPIProgress) OnStart(ctx context.Context, task *ytdlp.Task) {
	if info, ok := GetTask(p.taskID); ok {
		info.UpdateStatus(TaskStatusRunning)
		info.UpdatePhase("downloading")
	}
}

func (p *ytdlpAPIProgress) OnProgress(ctx context.Context, task *ytdlp.Task, status string) {
	if info, ok := GetTask(p.taskID); ok {
		if strings.HasPrefix(strings.ToLower(status), "transferred") {
			info.UpdatePhase("uploading")
		} else {
			info.UpdatePhase("downloading")
		}
	}
}

func (p *ytdlpAPIProgress) OnDone(ctx context.Context, task *ytdlp.Task, err error) {
	finishAPITask(p.taskID, err)
}

type aria2APIProgress struct{ taskID string }

func newAria2APIProgress(taskID string) aria2dl.ProgressTracker {
	return &aria2APIProgress{taskID: taskID}
}

func (p *aria2APIProgress) OnStart(ctx context.Context, task *aria2dl.Task) {
	if info, ok := GetTask(p.taskID); ok {
		info.UpdateStatus(TaskStatusRunning)
		info.UpdatePhase("downloading")
	}
}

func (p *aria2APIProgress) OnProgress(ctx context.Context, task *aria2dl.Task, status *aria2.Status) {
	if info, ok := GetTask(p.taskID); ok {
		total, _ := strconv.ParseInt(status.TotalLength, 10, 64)
		completed, _ := strconv.ParseInt(status.CompletedLength, 10, 64)
		info.UpdatePhase("downloading")
		info.UpdateDownloadProgress(completed, total)
	}
}

func (p *aria2APIProgress) OnDone(ctx context.Context, task *aria2dl.Task, err error) {
	finishAPITask(p.taskID, err)
}

type parsedAPIProgress struct{ taskID string }

func newParsedAPIProgress(taskID string) parsed.ProgressTracker {
	return &parsedAPIProgress{taskID: taskID}
}

func (p *parsedAPIProgress) OnStart(ctx context.Context, info parsed.TaskInfo) {
	if task, ok := GetTask(p.taskID); ok {
		task.UpdateStatus(TaskStatusRunning)
		task.UpdatePhase("downloading")
		task.TotalFiles = int(info.TotalResources())
		task.UpdateDownloadProgress(info.DownloadedBytes(), info.TotalBytes())
	}
}

func (p *parsedAPIProgress) OnProgress(ctx context.Context, info parsed.TaskInfo) {
	if task, ok := GetTask(p.taskID); ok {
		task.UpdatePhase("downloading")
		task.TotalFiles = int(info.TotalResources())
		task.DownloadedFiles = int(info.Downloaded())
		task.UpdateDownloadProgress(info.DownloadedBytes(), info.TotalBytes())
	}
}

func (p *parsedAPIProgress) OnDone(ctx context.Context, info parsed.TaskInfo, err error) {
	finishAPITask(p.taskID, err)
}

type tfileAPIProgress struct{ taskID string }

func newTFileAPIProgress(taskID string) tfile.ProgressTracker {
	return &tfileAPIProgress{taskID: taskID}
}

func (p *tfileAPIProgress) OnStart(ctx context.Context, info tfile.TaskInfo) {
	if task, ok := GetTask(p.taskID); ok {
		task.UpdateStatus(TaskStatusRunning)
		task.UpdatePhase("downloading")
		task.TotalFiles = 1
		task.UpdateDownloadProgress(0, info.FileSize())
	}
}

func (p *tfileAPIProgress) OnProgress(ctx context.Context, info tfile.TaskInfo, downloaded, total int64) {
	if task, ok := GetTask(p.taskID); ok {
		task.UpdatePhase("downloading")
		task.UpdateDownloadProgress(downloaded, total)
	}
}

func (p *tfileAPIProgress) OnDone(ctx context.Context, info tfile.TaskInfo, err error) {
	finishAPITask(p.taskID, err)
}

type batchTFileAPIProgress struct{ taskID string }

func newBatchTFileAPIProgress(taskID string) batchtfile.ProgressTracker {
	return &batchTFileAPIProgress{taskID: taskID}
}

func (p *batchTFileAPIProgress) OnStart(ctx context.Context, info batchtfile.TaskInfo) {
	if task, ok := GetTask(p.taskID); ok {
		task.UpdateStatus(TaskStatusRunning)
		task.UpdatePhase("downloading")
		task.TotalFiles = info.Count()
		task.UpdateDownloadProgress(0, info.TotalSize())
	}
}

func (p *batchTFileAPIProgress) OnProgress(ctx context.Context, info batchtfile.TaskInfo) {
	if task, ok := GetTask(p.taskID); ok {
		task.UpdatePhase("downloading")
		task.TotalFiles = info.Count()
		task.UpdateDownloadProgress(info.Downloaded(), info.TotalSize())
	}
}

func (p *batchTFileAPIProgress) OnDone(ctx context.Context, info batchtfile.TaskInfo, err error) {
	finishAPITask(p.taskID, err)
}

type telegraphAPIProgress struct{ taskID string }

func newTelegraphAPIProgress(taskID string) tphtask.ProgressTracker {
	return &telegraphAPIProgress{taskID: taskID}
}

func (p *telegraphAPIProgress) OnStart(ctx context.Context, info tphtask.TaskInfo) {
	if task, ok := GetTask(p.taskID); ok {
		task.UpdateStatus(TaskStatusRunning)
		task.UpdatePhase("downloading")
		task.TotalFiles = info.TotalPics()
	}
}

func (p *telegraphAPIProgress) OnProgress(ctx context.Context, info tphtask.TaskInfo) {
	if task, ok := GetTask(p.taskID); ok {
		task.UpdatePhase("downloading")
		task.DownloadedFiles = int(info.Downloaded())
	}
}

func (p *telegraphAPIProgress) OnDone(ctx context.Context, info tphtask.TaskInfo, err error) {
	finishAPITask(p.taskID, err)
}

type transferAPIProgress struct{ taskID string }

func newTransferAPIProgress(taskID string) transfer.ProgressTracker {
	return &transferAPIProgress{taskID: taskID}
}

func (p *transferAPIProgress) OnStart(ctx context.Context, info transfer.TaskInfo) {
	if task, ok := GetTask(p.taskID); ok {
		task.UpdateStatus(TaskStatusRunning)
		task.UpdatePhase(info.Phase())
		task.TotalFiles = info.Count()
		task.UpdateDownloadProgress(info.Downloaded(), info.TotalSize())
		task.UpdateUploadProgress(info.Uploaded(), info.TotalSize())
		task.SetTransferMeta(info.SourceStorageName(), info.SourcePath(), info.TargetStorageName(), info.TargetPath())
	}
}

func (p *transferAPIProgress) OnProgress(ctx context.Context, info transfer.TaskInfo) {
	if task, ok := GetTask(p.taskID); ok {
		task.UpdatePhase(info.Phase())
		task.TotalFiles = info.Count()
		task.UpdateDownloadProgress(info.Downloaded(), info.TotalSize())
		task.UpdateUploadProgress(info.Uploaded(), info.TotalSize())
		task.SetTransferMeta(info.SourceStorageName(), info.SourcePath(), info.TargetStorageName(), info.TargetPath())
	}
}

func (p *transferAPIProgress) OnDone(ctx context.Context, info transfer.TaskInfo, err error) {
	if task, ok := GetTask(p.taskID); ok {
		task.UpdateDownloadProgress(info.Downloaded(), info.TotalSize())
		task.UpdateUploadProgress(info.Uploaded(), info.TotalSize())
		task.SetTransferMeta(info.SourceStorageName(), info.SourcePath(), info.TargetStorageName(), info.TargetPath())
	}
	finishAPITask(p.taskID, err)
}

func finishAPITask(taskID string, err error) {
	info, ok := GetTask(taskID)
	if !ok {
		return
	}
	if err != nil {
		if errors.Is(err, context.Canceled) {
			if info.Status != TaskStatusPaused {
				info.UpdateStatus(TaskStatusCancelled)
				info.UpdatePhase("cancelled")
			}
			return
		}
		info.SetError(err.Error())
		return
	}
	info.UpdateStatus(TaskStatusCompleted)
	info.UpdatePhase("completed")
}
