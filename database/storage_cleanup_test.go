package database

import (
	"context"
	"testing"

	"github.com/krau/SaveAny-Bot/pkg/rule"
	"gorm.io/gorm"
)

func TestClearStorageReferences(t *testing.T) {
	ctx := context.Background()
	openTestDB(t)

	user := User{
		ChatID:         1001,
		DefaultStorage: "removed",
		Silent:         true,
	}
	if err := db.WithContext(ctx).Create(&user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	dir := Dir{UserID: user.ID, StorageName: "removed", Path: "/old"}
	otherDir := Dir{UserID: user.ID, StorageName: "kept", Path: "/new"}
	if err := db.WithContext(ctx).Create(&dir).Error; err != nil {
		t.Fatalf("failed to create removed dir: %v", err)
	}
	if err := db.WithContext(ctx).Create(&otherDir).Error; err != nil {
		t.Fatalf("failed to create kept dir: %v", err)
	}
	user.DefaultDir = dir.ID
	if err := db.WithContext(ctx).Save(&user).Error; err != nil {
		t.Fatalf("failed to update user default dir: %v", err)
	}
	if err := db.WithContext(ctx).Create(&Rule{
		UserID:      user.ID,
		Type:        rule.FileNameRegex.String(),
		Data:        ".*",
		StorageName: "removed",
		DirPath:     "/old",
	}).Error; err != nil {
		t.Fatalf("failed to create rule: %v", err)
	}

	if err := ClearStorageReferences(ctx, "removed"); err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}

	var gotUser User
	if err := db.WithContext(ctx).First(&gotUser, user.ID).Error; err != nil {
		t.Fatalf("failed to load user: %v", err)
	}
	if gotUser.DefaultStorage != "" || gotUser.DefaultDir != 0 || gotUser.Silent {
		t.Fatalf("expected user defaults and silent to be cleared, got storage=%q dir=%d silent=%v", gotUser.DefaultStorage, gotUser.DefaultDir, gotUser.Silent)
	}

	var dirCount int64
	if err := db.WithContext(ctx).Model(&Dir{}).Where("storage_name = ?", "removed").Count(&dirCount).Error; err != nil {
		t.Fatalf("failed to count removed dirs: %v", err)
	}
	if dirCount != 0 {
		t.Fatalf("expected removed storage dirs to be deleted, got %d", dirCount)
	}
	if err := db.WithContext(ctx).Model(&Dir{}).Where("storage_name = ?", "kept").Count(&dirCount).Error; err != nil {
		t.Fatalf("failed to count kept dirs: %v", err)
	}
	if dirCount != 1 {
		t.Fatalf("expected kept storage dir to remain, got %d", dirCount)
	}

	var gotRule Rule
	if err := db.WithContext(ctx).Where("user_id = ?", user.ID).First(&gotRule).Error; err != nil {
		t.Fatalf("failed to load rule: %v", err)
	}
	if gotRule.StorageName != rule.RuleStorNameChosen {
		t.Fatalf("expected rule storage to be reset to %q, got %q", rule.RuleStorNameChosen, gotRule.StorageName)
	}
}

func openTestDB(t *testing.T) {
	t.Helper()
	var err error
	db, err = gorm.Open(GetDialect(t.TempDir()+"/saveany.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	if err := db.AutoMigrate(&User{}, &Dir{}, &Rule{}, &WatchChat{}); err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}
	t.Cleanup(func() { db = nil })
}
