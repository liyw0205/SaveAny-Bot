package api

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"
)

// TaskProgressInfo 存储任务的进度信息
type TaskProgressInfo struct {
	TaskID          string
	Type            string
	Status          TaskStatus
	Title           string
	TotalBytes      int64
	DownloadedBytes int64
	UploadedBytes   int64
	TotalFiles      int
	DownloadedFiles int
	Storage         string
	Path            string
	SourceStorage   string
	SourcePath      string
	TargetStorage   string
	TargetPath      string
	Phase           string
	Error           string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Webhook         string
	Request         *CreateTaskRequest
}

// progressStore 存储所有 API 任务的进度信息
type progressStore struct {
	mu       sync.RWMutex
	tasks    map[string]*TaskProgressInfo
	controls map[string]TaskControl
}

var store = &progressStore{
	tasks:    make(map[string]*TaskProgressInfo),
	controls: make(map[string]TaskControl),
}

type TaskControl interface {
	UpdateTargetPath(path string)
}

// RegisterTask 注册一个新的 API 任务
func RegisterTask(taskID, taskType, storage, path, title, webhook string, reqs ...*CreateTaskRequest) *TaskProgressInfo {
	info := &TaskProgressInfo{
		TaskID:    taskID,
		Type:      taskType,
		Status:    TaskStatusQueued,
		Title:     title,
		Storage:   storage,
		Path:      path,
		Phase:     "queued",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Webhook:   webhook,
	}
	if len(reqs) > 0 && reqs[0] != nil {
		info.Request = cloneCreateTaskRequest(reqs[0])
	}

	store.mu.Lock()
	store.tasks[taskID] = info
	store.mu.Unlock()

	return info
}

// GetTask 获取任务进度信息
func GetTask(taskID string) (*TaskProgressInfo, bool) {
	store.mu.RLock()
	defer store.mu.RUnlock()
	info, ok := store.tasks[taskID]
	return info, ok
}

// GetAllTasks 获取所有任务
func GetAllTasks() []*TaskProgressInfo {
	store.mu.RLock()
	defer store.mu.RUnlock()

	tasks := make([]*TaskProgressInfo, 0, len(store.tasks))
	for _, info := range store.tasks {
		tasks = append(tasks, info)
	}
	return tasks
}

// DeleteTask 删除任务记录
func DeleteTask(taskID string) {
	store.mu.Lock()
	defer store.mu.Unlock()
	delete(store.tasks, taskID)
	delete(store.controls, taskID)
}

func RegisterTaskControl(taskID string, control TaskControl) {
	if control == nil {
		return
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	store.controls[taskID] = control
}

func GetTaskControl(taskID string) (TaskControl, bool) {
	store.mu.RLock()
	defer store.mu.RUnlock()
	control, ok := store.controls[taskID]
	return control, ok
}

// UpdateStatus 更新任务状态
func (t *TaskProgressInfo) UpdateStatus(status TaskStatus) {
	t.Status = status
	t.UpdatedAt = time.Now()
}

// SetError 设置错误信息
func (t *TaskProgressInfo) SetError(err string) {
	t.Error = err
	t.Status = TaskStatusFailed
	t.Phase = "failed"
	t.UpdatedAt = time.Now()
}

func (t *TaskProgressInfo) SetTransferMeta(sourceStorage, sourcePath, targetStorage, targetPath string) {
	t.SourceStorage = sourceStorage
	t.SourcePath = sourcePath
	t.TargetStorage = targetStorage
	t.TargetPath = targetPath
	t.Path = targetPath
	t.UpdatedAt = time.Now()
}

func (t *TaskProgressInfo) UpdatePhase(phase string) {
	t.Phase = phase
	t.UpdatedAt = time.Now()
}

func (t *TaskProgressInfo) UpdateDownloadProgress(downloadedBytes, totalBytes int64) {
	if totalBytes > 0 {
		t.TotalBytes = totalBytes
	}
	atomic.StoreInt64(&t.DownloadedBytes, downloadedBytes)
	t.UpdatedAt = time.Now()
}

func (t *TaskProgressInfo) UpdateUploadProgress(uploadedBytes, totalBytes int64) {
	if totalBytes > 0 {
		t.TotalBytes = totalBytes
	}
	atomic.StoreInt64(&t.UploadedBytes, uploadedBytes)
	t.UpdatedAt = time.Now()
}

func (t *TaskProgressInfo) UpdateTargetPath(path string) {
	t.TargetPath = path
	t.Path = path
	t.UploadedBytes = 0
	t.UpdatedAt = time.Now()
}

// ProgressTracker 用于 API 任务的进度追踪
type ProgressTracker struct {
	info *TaskProgressInfo
}

// NewProgressTracker 创建新的进度追踪器
func NewProgressTracker(taskID, taskType, storage, path, title, webhook string) *ProgressTracker {
	info := RegisterTask(taskID, taskType, storage, path, title, webhook)
	return &ProgressTracker{info: info}
}

// OnStart 任务开始
func (p *ProgressTracker) OnStart(totalBytes int64, totalFiles int) {
	p.info.Status = TaskStatusRunning
	p.info.TotalBytes = totalBytes
	p.info.TotalFiles = totalFiles
	p.info.Phase = "running"
	p.info.UpdatedAt = time.Now()
}

// OnProgress 进度更新
func (p *ProgressTracker) OnProgress(downloadedBytes int64, downloadedFiles int) {
	atomic.StoreInt64(&p.info.DownloadedBytes, downloadedBytes)
	p.info.DownloadedFiles = downloadedFiles
	p.info.UpdatedAt = time.Now()
}

// OnDone 任务完成
func (p *ProgressTracker) OnDone(err error) {
	if err != nil {
		p.info.Status = TaskStatusFailed
		p.info.Error = err.Error()
		p.info.Phase = "failed"
	} else {
		p.info.Status = TaskStatusCompleted
		p.info.Phase = "completed"
	}
	p.info.UpdatedAt = time.Now()
}

// GetInfo 获取任务信息
func (p *ProgressTracker) GetInfo() *TaskProgressInfo {
	return p.info
}

// UpdateProgressBytes 更新下载字节数
func (p *ProgressTracker) UpdateProgressBytes(bytes int64) {
	atomic.StoreInt64(&p.info.DownloadedBytes, bytes)
	p.info.UpdatedAt = time.Now()
}

// UpdateProgressFiles 更新下载文件数
func (p *ProgressTracker) UpdateProgressFiles(files int) {
	p.info.DownloadedFiles = files
	p.info.UpdatedAt = time.Now()
}

func cloneCreateTaskRequest(req *CreateTaskRequest) *CreateTaskRequest {
	if req == nil {
		return nil
	}
	cloned := *req
	if req.Params != nil {
		cloned.Params = append(json.RawMessage(nil), req.Params...)
	}
	return &cloned
}
