package storage

import (
	"testing"

	storenum "github.com/krau/SaveAny-Bot/pkg/enums/storage"
)

func TestFillDefaultBasePath(t *testing.T) {
	tests := []struct {
		name     string
		cfg      StorageConfig
		expected string
	}{
		{name: "local", cfg: &LocalStorageConfig{BaseConfig: BaseConfig{Name: "local", Type: storenum.Local.String()}}, expected: "./downloads"},
		{name: "webdav", cfg: &WebdavStorageConfig{BaseConfig: BaseConfig{Name: "webdav", Type: storenum.Webdav.String()}}, expected: "/telegram"},
		{name: "alist", cfg: &AlistStorageConfig{BaseConfig: BaseConfig{Name: "alist", Type: storenum.Alist.String()}}, expected: "/telegram"},
		{name: "minio", cfg: &MinioStorageConfig{BaseConfig: BaseConfig{Name: "minio", Type: storenum.Minio.String()}}, expected: "telegram"},
		{name: "s3", cfg: &S3StorageConfig{BaseConfig: BaseConfig{Name: "s3", Type: storenum.S3.String()}}, expected: "telegram"},
		{name: "rclone", cfg: &RcloneStorageConfig{BaseConfig: BaseConfig{Name: "rclone", Type: storenum.Rclone.String()}}, expected: "/telegram"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fillDefaultBasePath(tt.cfg)
			got := basePath(tt.cfg)
			if got != tt.expected {
				t.Fatalf("expected base path %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestFillDefaultBasePathKeepsExistingValue(t *testing.T) {
	cfg := &LocalStorageConfig{
		BaseConfig: BaseConfig{Name: "local", Type: storenum.Local.String()},
		BasePath:   "/custom",
	}

	fillDefaultBasePath(cfg)

	if cfg.BasePath != "/custom" {
		t.Fatalf("expected existing base path to remain, got %q", cfg.BasePath)
	}
}

func basePath(cfg StorageConfig) string {
	switch c := cfg.(type) {
	case *LocalStorageConfig:
		return c.BasePath
	case *WebdavStorageConfig:
		return c.BasePath
	case *AlistStorageConfig:
		return c.BasePath
	case *MinioStorageConfig:
		return c.BasePath
	case *S3StorageConfig:
		return c.BasePath
	case *RcloneStorageConfig:
		return c.BasePath
	default:
		return ""
	}
}
