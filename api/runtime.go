package api

import (
	"context"
	"fmt"
	"runtime"
	"slices"
	"strings"

	"github.com/blang/semver"
	"github.com/charmbracelet/log"
	"github.com/krau/SaveAny-Bot/common/i18n"
	"github.com/krau/SaveAny-Bot/config"
	"github.com/krau/SaveAny-Bot/database"
	"github.com/krau/SaveAny-Bot/storage"
	"github.com/unvgo/ghselfupdate"
)

type RuntimeApplyResult struct {
	Applied         []string `json:"applied"`
	RestartRequired []string `json:"restart_required"`
	Warnings        []string `json:"warnings,omitempty"`
}

type UpdateCheckResponse struct {
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version,omitempty"`
	LatestName     string `json:"latest_name,omitempty"`
	Found          bool   `json:"found"`
	HasUpdate      bool   `json:"has_update"`
	ReleaseURL     string `json:"release_url,omitempty"`
	ReleaseNotes   string `json:"release_notes,omitempty"`
	Platform       string `json:"platform"`
	Message        string `json:"message,omitempty"`
}

func ApplyRuntimeConfig(ctx context.Context, configPath string, before config.Config) RuntimeApplyResult {
	result := RuntimeApplyResult{
		Applied: []string{"配置文件"},
	}
	after := config.C()

	if err := applyLogLevel(after.Log.Level); err != nil {
		result.Warnings = append(result.Warnings, err.Error())
	} else {
		result.Applied = append(result.Applied, "日志级别")
	}

	i18n.Init(after.Lang)
	result.Applied = append(result.Applied, "语言")

	if database.Ready() {
		if dbPathChanged(before, after) {
			result.RestartRequired = append(result.RestartRequired, "数据库路径")
		} else if err := database.SyncUsers(ctx); err != nil {
			result.Warnings = append(result.Warnings, "同步用户失败: "+err.Error())
		} else {
			result.Applied = append(result.Applied, "用户配置")
		}
	}

	if err := storage.ReloadStorages(ctx); err != nil {
		result.Warnings = append(result.Warnings, "部分存储加载失败: "+err.Error())
	}
	result.Applied = append(result.Applied, "存储配置")

	result.Applied = append(result.Applied, "全局 HTTP 代理", "API Token")
	result.RestartRequired = append(result.RestartRequired, restartRequiredChanges(before, after)...)
	result.RestartRequired = compactStrings(result.RestartRequired)
	result.Applied = compactStrings(result.Applied)
	return result
}

func CheckUpdate() (*UpdateCheckResponse, error) {
	resp := &UpdateCheckResponse{
		CurrentVersion: config.Version,
		Platform:       runtime.GOOS + "/" + runtime.GOARCH,
	}
	current, err := semver.ParseTolerant(strings.TrimPrefix(config.Version, "v"))
	if err != nil {
		resp.Message = "当前版本不是发布版本，无法准确比较。"
	}

	latest, found, err := ghselfupdate.DetectLatest(config.GitRepo)
	if err != nil {
		return nil, err
	}
	resp.Found = found
	if !found {
		resp.Message = "没有找到 release。"
		return resp, nil
	}

	resp.LatestVersion = latest.Version.String()
	resp.LatestName = latest.Name
	resp.ReleaseNotes = latest.ReleaseNotes
	resp.ReleaseURL = latest.URL
	if err == nil {
		resp.HasUpdate = latest.Version.GT(current)
		if latest.Version.Equals(current) || latest.Version.LT(current) {
			resp.Message = "当前已经是最新版本。"
		}
	}
	return resp, nil
}

func applyLogLevel(level string) error {
	parsed, err := log.ParseLevel(strings.TrimSpace(level))
	if err != nil {
		return fmt.Errorf("日志级别无效: %w", err)
	}
	log.Default().SetLevel(parsed)
	return nil
}

func restartRequiredChanges(before, after config.Config) []string {
	var fields []string
	if before.Lang == "" && before.DB.Path == "" {
		return fields
	}
	if before.Workers != 0 && before.Workers != after.Workers {
		fields = append(fields, "Workers 并发数")
	}
	if before.API.Enable != after.API.Enable || before.API.Host != after.API.Host || before.API.Port != after.API.Port {
		fields = append(fields, "HTTP API 监听地址/端口")
	}
	if before.Telegram.Token != after.Telegram.Token ||
		before.Telegram.AppID != after.Telegram.AppID ||
		before.Telegram.AppHash != after.Telegram.AppHash ||
		before.Telegram.Proxy != after.Telegram.Proxy ||
		before.Telegram.Userbot != after.Telegram.Userbot {
		fields = append(fields, "Telegram Bot/Userbot 连接")
	}
	if before.Parser.PluginEnable != after.Parser.PluginEnable || !slices.Equal(before.Parser.PluginDirs, after.Parser.PluginDirs) {
		fields = append(fields, "Parser 插件加载")
	}
	if before.Cache != after.Cache {
		fields = append(fields, "缓存参数")
	}
	if before.Hook != after.Hook {
		fields = append(fields, "任务 Hook")
	}
	return fields
}

func dbPathChanged(before, after config.Config) bool {
	return before.DB.Path != "" && before.DB.Path != after.DB.Path
}

func compactStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
