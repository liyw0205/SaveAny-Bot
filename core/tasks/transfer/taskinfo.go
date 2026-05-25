package transfer

type TaskElementInfo interface {
	FileName() string
	FileSize() int64
	GetSourcePath() string
	SourceStorageName() string
}

func (e *TaskElement) FileName() string {
	return e.FileInfo.Name
}

func (e *TaskElement) FileSize() int64 {
	return e.FileInfo.Size
}

func (e *TaskElement) GetSourcePath() string {
	return e.SourcePath
}

func (e *TaskElement) SourceStorageName() string {
	return e.SourceStorage.Name()
}

type TaskInfo interface {
	TaskID() string
	TotalSize() int64
	Downloaded() int64
	Uploaded() int64
	Count() int
	Processing() []TaskElementInfo
	FailedFiles() []string
	Phase() string
	SourceStorageName() string
	SourcePath() string
	TargetStorageName() string
	TargetPath() string
}

func (t *Task) TotalSize() int64 {
	return t.totalSize
}

func (t *Task) Downloaded() int64 {
	return t.downloaded.Load()
}

func (t *Task) Uploaded() int64 {
	return t.uploaded.Load() + t.uploadedInFlight()
}

func (t *Task) Count() int {
	return len(t.elems)
}

func (t *Task) Processing() []TaskElementInfo {
	t.processingMu.RLock()
	defer t.processingMu.RUnlock()

	result := make([]TaskElementInfo, 0, len(t.processing))
	for _, elem := range t.processing {
		result = append(result, elem)
	}
	return result
}

func (t *Task) FailedFiles() []string {
	t.processingMu.RLock()
	defer t.processingMu.RUnlock()

	result := make([]string, 0, len(t.failed))
	for id := range t.failed {
		// Find the element by ID
		for _, elem := range t.elems {
			if elem.ID == id {
				result = append(result, elem.FileInfo.Name)
				break
			}
		}
	}
	return result
}

func (t *Task) Phase() string {
	t.phaseMu.RLock()
	defer t.phaseMu.RUnlock()
	return t.phase
}

func (t *Task) SourceStorageName() string {
	if len(t.elems) == 0 {
		return ""
	}
	return t.elems[0].SourceStorage.Name()
}

func (t *Task) SourcePath() string {
	if len(t.elems) == 0 {
		return ""
	}
	return t.elems[0].SourcePath
}

func (t *Task) TargetStorageName() string {
	if len(t.elems) == 0 {
		return ""
	}
	return t.elems[0].TargetStorage.Name()
}

func (t *Task) TargetPath() string {
	return t.currentTargetPath()
}
