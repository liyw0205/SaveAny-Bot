package storage

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/charmbracelet/log"
	"github.com/krau/SaveAny-Bot/config"
	storenum "github.com/krau/SaveAny-Bot/pkg/enums/storage"
)

var UserStorages = make(map[int64][]Storage)
var storagesMu sync.RWMutex

// GetStorageByName returns storage by name from cache or creates new one
// It should NOT be used to get storage for user, use GetStorageByUserIDAndName instead
func GetStorageByName(ctx context.Context, name string) (Storage, error) {
	if name == "" {
		return nil, ErrStorageNameEmpty
	}

	storagesMu.RLock()
	storage, ok := Storages[name]
	storagesMu.RUnlock()
	if ok {
		return storage, nil
	}

	cfg := config.C().GetStorageByName(name)
	if cfg == nil {
		return nil, fmt.Errorf("未找到存储 %s", name)
	}

	storage, err := NewStorage(ctx, cfg)
	if err != nil {
		return nil, err
	}

	storagesMu.Lock()
	if existing, ok := Storages[name]; ok {
		storagesMu.Unlock()
		return existing, nil
	}
	Storages[name] = storage
	storagesMu.Unlock()
	return storage, nil
}

// 检查 user 是否可用指定的 storage, 若不可用则返回未找到错误
func GetStorageByUserIDAndName(ctx context.Context, chatID int64, name string) (Storage, error) {
	if name == "" {
		return nil, ErrStorageNameEmpty
	}

	if !config.C().HasStorage(chatID, name) {
		return nil, fmt.Errorf("no storage %s for user %d", name, chatID)
	}

	return GetStorageByName(ctx, name)
}

func GetUserStorages(ctx context.Context, chatID int64) []Storage {
	if chatID <= 0 {
		return nil
	}
	storagesMu.RLock()
	if storages, ok := UserStorages[chatID]; ok {
		out := append([]Storage(nil), storages...)
		storagesMu.RUnlock()
		return out
	}
	storagesMu.RUnlock()

	var storages []Storage
	for _, name := range config.C().GetStorageNamesByUserID(chatID) {
		storage, err := GetStorageByName(ctx, name)
		if err != nil {
			continue
		}
		storages = append(storages, storage)
	}
	storagesMu.Lock()
	UserStorages[chatID] = storages
	out := append([]Storage(nil), storages...)
	storagesMu.Unlock()
	return out
}

func LoadStorages(ctx context.Context) {
	logger := log.FromContext(ctx)
	if err := ReloadStorages(ctx); err != nil {
		logger.Errorf("failed to load some storages: %v", err)
	}
}

func ReloadStorages(ctx context.Context) error {
	logger := log.FromContext(ctx)
	logger.Debug("loading storages...")

	nextStorages := make(map[string]Storage)
	var errs []error
	for _, cfg := range config.C().Storages {
		storage, err := NewStorage(ctx, cfg)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", cfg.GetName(), err))
			logger.Errorf("failed to load storage %s: %v", cfg.GetName(), err)
			continue
		}
		nextStorages[cfg.GetName()] = storage
	}

	nextUserStorages := make(map[int64][]Storage)
	for _, userID := range config.C().GetUsersID() {
		for _, name := range config.C().GetStorageNamesByUserID(userID) {
			storage, ok := nextStorages[name]
			if ok {
				nextUserStorages[userID] = append(nextUserStorages[userID], storage)
			}
		}
	}

	storagesMu.Lock()
	Storages = nextStorages
	UserStorages = nextUserStorages
	storagesMu.Unlock()

	logger.Infof("successfully loaded %d storages", len(nextStorages))
	return errors.Join(errs...)
}

func Snapshot() map[string]Storage {
	storagesMu.RLock()
	defer storagesMu.RUnlock()

	out := make(map[string]Storage, len(Storages))
	for name, storage := range Storages {
		out[name] = storage
	}
	return out
}

// GetTelegramStorageByUserID returns the first enabled Telegram storage for the user
func GetTelegramStorageByUserID(ctx context.Context, chatID int64) (Storage, error) {
	storages := GetUserStorages(ctx, chatID)
	for _, stor := range storages {
		if stor.Type() == storenum.Telegram {
			return stor, nil
		}
	}
	return nil, fmt.Errorf("no telegram storage found for user %d", chatID)
}
