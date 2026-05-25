package database

import (
	"context"
	"fmt"

	"github.com/krau/SaveAny-Bot/pkg/rule"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func CreateUser(ctx context.Context, chatID int64) error {
	if _, err := GetUserByChatID(ctx, chatID); err == nil {
		return nil
	}
	return db.Create(&User{ChatID: chatID}).Error
}

func GetAllUsers(ctx context.Context) ([]User, error) {
	var users []User
	err := db.WithContext(ctx).
		Preload(clause.Associations).
		Find(&users).Error
	return users, err
}

func GetUserByChatID(ctx context.Context, chatID int64) (*User, error) {
	var user User
	err := db.WithContext(ctx).
		Preload(clause.Associations).
		Where("chat_id = ?", chatID).First(&user).Error
	return &user, err
}

func UpdateUser(ctx context.Context, user *User) error {
	if _, err := GetUserByChatID(ctx, user.ChatID); err != nil {
		return err
	}
	return db.WithContext(ctx).Save(user).Error
}

func DeleteUser(ctx context.Context, user *User) error {
	return db.WithContext(ctx).
		Unscoped().
		Select(clause.Associations).
		Delete(user).Error
}

func GetUserByID(ctx context.Context, id uint) (*User, error) {
	var user User
	err := db.WithContext(ctx).
		Preload(clause.Associations).
		Where("id = ?", id).First(&user).Error
	return &user, err
}

func ClearStorageReferences(ctx context.Context, storageName string) error {
	if db == nil {
		return fmt.Errorf("database is not initialized")
	}
	if storageName == "" {
		return nil
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&User{}).
			Where("default_storage = ?", storageName).
			Updates(map[string]any{
				"default_storage": "",
				"default_dir":     0,
				"silent":          false,
			}).Error; err != nil {
			return err
		}
		if err := tx.Model(&User{}).
			Where("default_dir IN (?)", tx.Model(&Dir{}).Select("id").Where("storage_name = ?", storageName)).
			Update("default_dir", 0).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().
			Where("storage_name = ?", storageName).
			Delete(&Dir{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&Rule{}).
			Where("storage_name = ?", storageName).
			Update("storage_name", rule.RuleStorNameChosen).Error; err != nil {
			return err
		}
		return nil
	})
}

func RenameStorageReferences(ctx context.Context, oldName, newName string) error {
	if db == nil {
		return fmt.Errorf("database is not initialized")
	}
	if oldName == "" || newName == "" || oldName == newName {
		return nil
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&User{}).
			Where("default_storage = ?", oldName).
			Update("default_storage", newName).Error; err != nil {
			return err
		}
		if err := tx.Model(&Dir{}).
			Where("storage_name = ?", oldName).
			Update("storage_name", newName).Error; err != nil {
			return err
		}
		if err := tx.Model(&Rule{}).
			Where("storage_name = ?", oldName).
			Update("storage_name", newName).Error; err != nil {
			return err
		}
		return nil
	})
}
