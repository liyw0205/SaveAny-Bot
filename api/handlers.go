package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/krau/SaveAny-Bot/core"
	"github.com/krau/SaveAny-Bot/pkg/enums/tasktype"
	"github.com/krau/SaveAny-Bot/storage"
)

// Handlers 处理器结构体
type Handlers struct {
	factory *TaskFactory
}

// NewHandlers 创建处理器
func NewHandlers(factory *TaskFactory) *Handlers {
	return &Handlers{factory: factory}
}

// CreateTaskHandler 创建任务处理器
func (h *Handlers) CreateTaskHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only POST method is allowed")
		return
	}

	var req CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_request", "failed to decode request body: "+err.Error())
		return
	}

	// 验证请求
	if req.Type == "" {
		WriteError(w, http.StatusBadRequest, "invalid_request", "task type is required")
		return
	}

	if req.Storage == "" {
		WriteError(w, http.StatusBadRequest, "invalid_request", "storage is required")
		return
	}

	// 创建任务
	resp, err := h.factory.CreateTask(&req)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "task_creation_failed", err.Error())
		return
	}

	WriteJSON(w, http.StatusCreated, resp)
}

// ListTasksHandler 列出任务处理器
func (h *Handlers) ListTasksHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only GET method is allowed")
		return
	}

	tasks := GetAllTasks()
	response := make([]TaskInfoResponse, 0, len(tasks))

	for _, task := range tasks {
		info := convertTaskProgressToResponse(task)
		response = append(response, info)
	}

	WriteJSON(w, http.StatusOK, TasksListResponse{
		Tasks: response,
		Total: len(response),
	})
}

// GetTaskHandler 获取单个任务处理器
func (h *Handlers) GetTaskHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only GET method is allowed")
		return
	}

	taskID := extractTaskIDFromPath(r.URL.Path)
	if taskID == "" {
		WriteError(w, http.StatusBadRequest, "invalid_request", "task ID is required")
		return
	}

	task, ok := GetTask(taskID)
	if !ok {
		WriteError(w, http.StatusNotFound, "task_not_found", "task not found: "+taskID)
		return
	}

	resp := convertTaskProgressToResponse(task)
	WriteJSON(w, http.StatusOK, resp)
}

// CancelTaskHandler 取消任务处理器
func (h *Handlers) CancelTaskHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only DELETE method is allowed")
		return
	}

	taskID := extractTaskIDFromPath(r.URL.Path)
	if taskID == "" {
		WriteError(w, http.StatusBadRequest, "invalid_request", "task ID is required")
		return
	}

	task, ok := GetTask(taskID)
	if !ok {
		WriteError(w, http.StatusNotFound, "task_not_found", "task not found: "+taskID)
		return
	}

	if r.URL.Query().Get("record") == "1" || r.URL.Query().Get("record") == "true" {
		if task.Status == TaskStatusQueued || task.Status == TaskStatusRunning {
			_ = core.CancelTask(r.Context(), taskID)
		}
		DeleteTask(taskID)
		WriteJSON(w, http.StatusOK, map[string]string{"message": "task deleted successfully"})
		return
	}

	// 取消任务
	if err := core.CancelTask(r.Context(), taskID); err != nil {
		WriteError(w, http.StatusInternalServerError, "cancel_failed", "failed to cancel task: "+err.Error())
		return
	}

	task.UpdateStatus(TaskStatusCancelled)
	WriteJSON(w, http.StatusOK, map[string]string{"message": "task cancelled successfully"})
}

func (h *Handlers) PauseTaskHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		MethodNotAllowedHandler(w, r)
		return
	}
	taskID, _ := extractTaskIDAndAction(r.URL.Path)
	if taskID == "" {
		WriteError(w, http.StatusBadRequest, "invalid_request", "task ID is required")
		return
	}
	task, ok := GetTask(taskID)
	if !ok {
		WriteError(w, http.StatusNotFound, "task_not_found", "task not found: "+taskID)
		return
	}
	if task.Status == TaskStatusQueued || task.Status == TaskStatusRunning {
		if err := core.CancelTask(r.Context(), taskID); err != nil {
			WriteError(w, http.StatusInternalServerError, "pause_failed", "failed to pause task: "+err.Error())
			return
		}
	}
	task.UpdateStatus(TaskStatusPaused)
	task.UpdatePhase("paused")
	WriteJSON(w, http.StatusOK, map[string]string{"message": "task paused successfully"})
}

func (h *Handlers) RetryTaskHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		MethodNotAllowedHandler(w, r)
		return
	}
	taskID, _ := extractTaskIDAndAction(r.URL.Path)
	if taskID == "" {
		WriteError(w, http.StatusBadRequest, "invalid_request", "task ID is required")
		return
	}
	task, ok := GetTask(taskID)
	if !ok {
		WriteError(w, http.StatusNotFound, "task_not_found", "task not found: "+taskID)
		return
	}
	if task.Request == nil {
		WriteError(w, http.StatusBadRequest, "retry_unavailable", "task request data is not available")
		return
	}
	if task.Status != TaskStatusFailed && task.Status != TaskStatusCancelled && task.Status != TaskStatusPaused {
		WriteError(w, http.StatusBadRequest, "retry_unavailable", "only failed, cancelled, or paused tasks can be retried")
		return
	}
	resp, err := h.factory.CreateTask(cloneCreateTaskRequest(task.Request))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "retry_failed", err.Error())
		return
	}
	WriteJSON(w, http.StatusCreated, resp)
}

func (h *Handlers) UpdateTaskPathHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPatch {
		MethodNotAllowedHandler(w, r)
		return
	}
	taskID, _ := extractTaskIDAndAction(r.URL.Path)
	if taskID == "" {
		WriteError(w, http.StatusBadRequest, "invalid_request", "task ID is required")
		return
	}
	task, ok := GetTask(taskID)
	if !ok {
		WriteError(w, http.StatusNotFound, "task_not_found", "task not found: "+taskID)
		return
	}
	if task.Type != string(tasktype.TaskTypeTransfer) {
		WriteError(w, http.StatusBadRequest, "unsupported_task", "only transfer task path can be updated")
		return
	}
	var req UpdateTaskPathRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_request", "failed to decode request body: "+err.Error())
		return
	}
	req.Path = strings.TrimSpace(req.Path)
	if req.Path == "" {
		WriteError(w, http.StatusBadRequest, "invalid_request", "path is required")
		return
	}
	control, ok := GetTaskControl(taskID)
	if !ok {
		WriteError(w, http.StatusConflict, "control_unavailable", "task runtime control is not available")
		return
	}
	control.UpdateTargetPath(req.Path)
	task.UpdateTargetPath(req.Path)
	updateStoredTransferTargetPath(task, req.Path)
	WriteJSON(w, http.StatusOK, map[string]any{
		"message":     "task path updated",
		"target_path": req.Path,
	})
}

// ListStoragesHandler 列出存储处理器
func (h *Handlers) ListStoragesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only GET method is allowed")
		return
	}

	snapshot := storage.Snapshot()
	storages := make([]StorageInfo, 0, len(snapshot))
	for name, stor := range snapshot {
		storages = append(storages, StorageInfo{
			Name: name,
			Type: string(stor.Type()),
		})
	}

	WriteJSON(w, http.StatusOK, StoragesResponse{Storages: storages})
}

// GetTaskTypesHandler 获取支持的任务类型
func (h *Handlers) GetTaskTypesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only GET method is allowed")
		return
	}

	types := []tasktype.TaskType{
		tasktype.TaskTypeDirectlinks,
		tasktype.TaskTypeYtdlp,
		tasktype.TaskTypeAria2,
		tasktype.TaskTypeParseditem,
		tasktype.TaskTypeTgfiles,
		tasktype.TaskTypeTphpics,
		tasktype.TaskTypeTransfer,
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"types": types,
	})
}

// HealthCheckHandler 健康检查处理器
func (h *Handlers) HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

// extractTaskIDFromPath 从路径中提取任务 ID
// 路径格式: /api/v1/tasks/:id
func extractTaskIDFromPath(path string) string {
	taskID, _ := extractTaskIDAndAction(path)
	return taskID
}

func extractTaskIDAndAction(path string) (string, string) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 4 {
		return "", ""
	}
	action := ""
	if len(parts) > 4 {
		action = parts[4]
	}
	return parts[3], action
}

// convertTaskProgressToResponse 将任务进度转换为响应格式
func convertTaskProgressToResponse(task *TaskProgressInfo) TaskInfoResponse {
	resp := TaskInfoResponse{
		TaskID:        task.TaskID,
		Type:          tasktype.TaskType(task.Type),
		Status:        task.Status,
		Title:         task.Title,
		Storage:       task.Storage,
		Path:          task.Path,
		SourceStorage: task.SourceStorage,
		SourcePath:    task.SourcePath,
		TargetStorage: task.TargetStorage,
		TargetPath:    task.TargetPath,
		Phase:         task.Phase,
		Error:         task.Error,
		CreatedAt:     task.CreatedAt,
		UpdatedAt:     task.UpdatedAt,
	}

	// 计算进度
	if task.TotalBytes > 0 {
		downloaded := atomic.LoadInt64(&task.DownloadedBytes)
		uploaded := atomic.LoadInt64(&task.UploadedBytes)
		current := downloaded
		if uploaded > 0 || task.Phase == "uploading" {
			current = uploaded
		}
		percent := float64(current) * 100 / float64(task.TotalBytes)
		resp.Progress = &TaskProgress{
			TotalBytes:      task.TotalBytes,
			DownloadedBytes: downloaded,
			UploadedBytes:   uploaded,
			Percent:         percent,
		}
	}

	return resp
}

func updateStoredTransferTargetPath(info *TaskProgressInfo, targetPath string) {
	if info.Request == nil || info.Request.Params == nil {
		return
	}
	var params TransferParams
	if err := json.Unmarshal(info.Request.Params, &params); err != nil {
		return
	}
	params.TargetPath = targetPath
	data, err := json.Marshal(params)
	if err != nil {
		return
	}
	info.Request.Params = data
}

// NotFoundHandler 404 处理器
func NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	WriteError(w, http.StatusNotFound, "not_found", "endpoint not found: "+r.URL.Path)
}

// MethodNotAllowedHandler 405 处理器
func MethodNotAllowedHandler(w http.ResponseWriter, r *http.Request) {
	WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed: "+r.Method)
}
