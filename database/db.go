package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/log"
	"github.com/krau/SaveAny-Bot/config"
	"github.com/krau/SaveAny-Bot/pkg/rule"
	"gorm.io/gorm"
	glogger "gorm.io/gorm/logger"
)

var db *gorm.DB

func Init(ctx context.Context) {
	if err := Open(ctx); err != nil {
		log.FromContext(ctx).Fatal("Database initialization failed", "error", err)
	}
}

func Open(ctx context.Context) error {
	logger := log.FromContext(ctx)
	if err := os.MkdirAll(filepath.Dir(config.C().DB.Path), 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}
	var err error
	db, err = gorm.Open(GetDialect(config.C().DB.Path), &gorm.Config{
		Logger: glogger.New(logger, glogger.Config{
			Colorful:                  true,
			SlowThreshold:             time.Second * 5,
			LogLevel:                  glogger.Error,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      true,
		}),
		PrepareStmt: true,
	})
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	logger.Debug("Database connected")
	if err := db.AutoMigrate(&User{}, &Dir{}, &Rule{}, &WatchChat{}); err != nil {
		return fmt.Errorf("database migration failed; if upgrading from an old version, try deleting the database file and retrying: %w", err)
	}
	if err := SyncUsers(ctx); err != nil {
		return fmt.Errorf("failed to sync users: %w", err)
	}
	logger.Debug("Database migrated")
	logger.Info("Database initialized")
	return nil
}

func Ready() bool {
	return db != nil
}

func SyncUsers(ctx context.Context) error {
	if db == nil {
		return fmt.Errorf("database is not initialized")
	}
	logger := log.FromContext(ctx)
	dbUsers, err := GetAllUsers(ctx)
	if err != nil {
		return fmt.Errorf("failed to get users: %w", err)
	}

	dbUserMap := make(map[int64]User)
	for _, u := range dbUsers {
		dbUserMap[u.ChatID] = u
	}

	cfgUserMap := make(map[int64]struct{})
	for _, u := range config.C().Users {
		cfgUserMap[u.ID] = struct{}{}
	}

	for cfgID := range cfgUserMap {
		if _, exists := dbUserMap[cfgID]; !exists {
			if err := CreateUser(ctx, cfgID); err != nil {
				return fmt.Errorf("failed to create user %d: %w", cfgID, err)
			}
			logger.Infof("Created user from config: %d", cfgID)
		}
	}

	for dbID, dbUser := range dbUserMap {
		if _, exists := cfgUserMap[dbID]; !exists {
			if err := DeleteUser(ctx, &dbUser); err != nil {
				return fmt.Errorf("failed to delete user %d: %w", dbID, err)
			}
			logger.Infof("Deleted user not present in config: %d", dbID)
		}
	}

	if err := cleanupUnavailableStorageReferences(ctx); err != nil {
		return err
	}

	return nil
}

func cleanupUnavailableStorageReferences(ctx context.Context) error {
	knownStorages := make(map[string]struct{}, len(config.C().Storages))
	for _, storage := range config.C().Storages {
		knownStorages[storage.GetName()] = struct{}{}
	}

	names := make(map[string]struct{})
	for _, user := range config.C().Users {
		for _, storageName := range user.Storages {
			if _, ok := knownStorages[storageName]; !ok && storageName != "" {
				names[storageName] = struct{}{}
			}
		}
	}

	var users []User
	if err := db.WithContext(ctx).
		Where("default_storage <> ''").
		Find(&users).Error; err != nil {
		return fmt.Errorf("failed to find users with default storage: %w", err)
	}
	for _, user := range users {
		if _, ok := knownStorages[user.DefaultStorage]; !ok {
			names[user.DefaultStorage] = struct{}{}
		}
	}

	var dirs []Dir
	if err := db.WithContext(ctx).
		Select("storage_name").
		Group("storage_name").
		Find(&dirs).Error; err != nil {
		return fmt.Errorf("failed to find storage dirs: %w", err)
	}
	for _, dir := range dirs {
		if _, ok := knownStorages[dir.StorageName]; !ok && dir.StorageName != "" {
			names[dir.StorageName] = struct{}{}
		}
	}

	var rules []Rule
	if err := db.WithContext(ctx).
		Select("storage_name").
		Where("storage_name <> '' AND storage_name <> ?", rule.RuleStorNameChosen).
		Group("storage_name").
		Find(&rules).Error; err != nil {
		return fmt.Errorf("failed to find storage rules: %w", err)
	}
	for _, rule := range rules {
		if _, ok := knownStorages[rule.StorageName]; !ok {
			names[rule.StorageName] = struct{}{}
		}
	}

	for name := range names {
		if err := ClearStorageReferences(ctx, name); err != nil {
			return fmt.Errorf("failed to clear references for storage %s: %w", name, err)
		}
		log.FromContext(ctx).Warnf("Cleared references to unavailable storage: %s", name)
	}
	return nil
}
