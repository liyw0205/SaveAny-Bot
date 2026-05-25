package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/krau/SaveAny-Bot/common/logbuffer"
	"github.com/krau/SaveAny-Bot/common/utils/netutil"
	"github.com/krau/SaveAny-Bot/config"
	"github.com/krau/SaveAny-Bot/database"
	"github.com/krau/SaveAny-Bot/pkg/rule"
)

type ConfigEditor struct {
	ctx              context.Context
	configPath       string
	autoOpenDatabase bool
	mu               sync.Mutex
}

type ConfigEditorOption func(*ConfigEditor)

func WithConfigEditorAutoOpenDatabase(autoOpen bool) ConfigEditorOption {
	return func(editor *ConfigEditor) {
		editor.autoOpenDatabase = autoOpen
	}
}

func RegisterConfigEditorRoutes(ctx context.Context, mux *http.ServeMux, configPath string, opts ...ConfigEditorOption) *ConfigEditor {
	editor := &ConfigEditor{
		ctx:        ctx,
		configPath: config.ResolveConfigFilePath(configPath),
	}
	for _, opt := range opts {
		opt(editor)
	}
	mux.HandleFunc("/config", editor.WebHandler)
	mux.HandleFunc("/config/", editor.WebHandler)
	mux.HandleFunc("/api/v1/config", editor.ConfigHandler)
	mux.HandleFunc("/api/v1/config/apply", editor.ApplyConfigHandler)
	mux.HandleFunc("/api/v1/config/schema", editor.SchemaHandler)
	mux.HandleFunc("/api/v1/config/logs", editor.LogsHandler)
	mux.HandleFunc("/api/v1/config/proxy-test", editor.ProxyTestHandler)
	mux.HandleFunc("/api/v1/config/update-check", editor.UpdateCheckHandler)
	mux.HandleFunc("/api/v1/config/rules/apply", editor.ApplyRuleHandler)
	mux.HandleFunc("/api/v1/config/rules", editor.RulesHandler)
	mux.HandleFunc("/api/v1/config/rules/", editor.RuleByIDHandler)
	return editor
}

func (e *ConfigEditor) WebHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		MethodNotAllowedHandler(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(configWebHTML)
}

func (e *ConfigEditor) ConfigHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		file, err := config.LoadEditableConfig(e.configPath)
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "config_load_failed", err.Error())
			return
		}
		WriteJSON(w, http.StatusOK, map[string]any{
			"path":    file.Path,
			"exists":  file.Exists,
			"config":  file.Config,
			"message": "保存后建议重启 bot 以完整应用存储、代理和高级配置。",
		})
	case http.MethodPut:
		cfg, err := decodeEditableConfigRequest(r)
		if err != nil {
			WriteError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		e.mu.Lock()
		defer e.mu.Unlock()
		storageChanges := e.storageChanges(cfg)
		if err := config.SaveEditableConfig(e.configPath, cfg); err != nil {
			WriteError(w, http.StatusBadRequest, "config_save_failed", err.Error())
			return
		}
		warnings := e.applyStorageReferenceChanges(r.Context(), storageChanges)
		WriteJSON(w, http.StatusOK, map[string]any{
			"message":  "config saved",
			"path":     e.configPath,
			"warnings": warnings,
		})
	default:
		MethodNotAllowedHandler(w, r)
	}
}

func (e *ConfigEditor) ApplyConfigHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		MethodNotAllowedHandler(w, r)
		return
	}
	before := config.C()
	e.mu.Lock()
	defer e.mu.Unlock()
	applyResult, err := e.reloadRuntimeConfig(r.Context(), before)
	if err != nil {
		WriteJSON(w, http.StatusAccepted, map[string]any{
			"message":      "runtime reload failed; restart the bot after fixing the config",
			"path":         e.configPath,
			"reload_error": err.Error(),
			"runtime":      applyResult,
		})
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"message": "config applied",
		"path":    e.configPath,
		"runtime": applyResult,
	})
}

func (e *ConfigEditor) LogsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		MethodNotAllowedHandler(w, r)
		return
	}
	limit := 200
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			WriteError(w, http.StatusBadRequest, "invalid_request", "limit must be a positive integer")
			return
		}
		limit = parsed
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"lines": logbuffer.Default().Lines(limit),
	})
}

func (e *ConfigEditor) UpdateCheckHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		MethodNotAllowedHandler(w, r)
		return
	}
	resp, err := CheckUpdate()
	if err != nil {
		WriteError(w, http.StatusBadGateway, "update_check_failed", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, resp)
}

func (e *ConfigEditor) ProxyTestHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		MethodNotAllowedHandler(w, r)
		return
	}
	var req ProxyTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_request", "failed to decode request body: "+err.Error())
		return
	}
	req.URL = strings.TrimSpace(req.URL)
	if req.URL == "" {
		WriteError(w, http.StatusBadRequest, "invalid_request", "proxy url is required")
		return
	}
	target := strings.TrimSpace(req.Target)
	if target == "" {
		target = "https://api.telegram.org"
	}
	client, err := netutil.NewProxyHTTPClient(req.URL)
	if err != nil {
		WriteJSON(w, http.StatusOK, ProxyTestResponse{OK: false, Message: err.Error(), Target: target})
		return
	}
	client.Timeout = 10 * time.Second
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	start := time.Now()
	resp, err := client.Do(httpReq)
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		WriteJSON(w, http.StatusOK, ProxyTestResponse{OK: false, MS: elapsed, Message: err.Error(), Target: target})
		return
	}
	defer resp.Body.Close()
	ok := resp.StatusCode >= 200 && resp.StatusCode < 500
	message := resp.Status
	if ok {
		message = "proxy reachable"
	}
	WriteJSON(w, http.StatusOK, ProxyTestResponse{OK: ok, MS: elapsed, Message: message, Target: target})
}

func (e *ConfigEditor) SchemaHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		MethodNotAllowedHandler(w, r)
		return
	}
	ruleTypes := make([]string, 0, len(rule.Values()))
	for _, ruleType := range rule.Values() {
		ruleTypes = append(ruleTypes, ruleType.String())
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"storage_types":        config.StorageTypeNames(),
		"storage_schemas":      config.StorageSchemas(),
		"rule_types":           ruleTypes,
		"rule_storage_chosen":  rule.RuleStorNameChosen,
		"rule_dir_new_album":   rule.RuleDirPathNewForAlbum,
		"database_ready":       database.Ready(),
		"config_path":          e.configPath,
		"config_reload_notice": "保存配置后建议重启 bot。",
	})
}

func (e *ConfigEditor) RulesHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if err := e.ensureDatabase(r.Context()); err != nil {
			WriteError(w, http.StatusServiceUnavailable, "database_unavailable", err.Error())
			return
		}
		users, err := database.GetAllUsers(r.Context())
		if err != nil {
			WriteError(w, http.StatusInternalServerError, "users_load_failed", err.Error())
			return
		}
		chatIDFilter, hasFilter, err := optionalChatID(r)
		if err != nil {
			WriteError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		respUsers := make([]configUserResponse, 0, len(users))
		for _, user := range users {
			if hasFilter && user.ChatID != chatIDFilter {
				continue
			}
			respUsers = append(respUsers, convertConfigUser(user))
		}
		WriteJSON(w, http.StatusOK, map[string]any{
			"users":    respUsers,
			"storages": configuredStorageNames(),
		})
	case http.MethodPost:
		if err := e.ensureDatabase(r.Context()); err != nil {
			WriteError(w, http.StatusServiceUnavailable, "database_unavailable", err.Error())
			return
		}
		var req createRuleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, http.StatusBadRequest, "invalid_request", "failed to decode request body: "+err.Error())
			return
		}
		normalized, err := normalizeCreateRuleRequest(req)
		if err != nil {
			WriteError(w, http.StatusBadRequest, "invalid_rule", err.Error())
			return
		}
		user, err := database.GetUserByChatID(r.Context(), normalized.ChatID)
		if err != nil {
			WriteError(w, http.StatusNotFound, "user_not_found", fmt.Sprintf("user %d is not in database; save config and reload first", normalized.ChatID))
			return
		}
		newRule := &database.Rule{
			UserID:      user.ID,
			Type:        normalized.Type,
			Data:        normalized.Data,
			StorageName: normalized.StorageName,
			DirPath:     normalized.DirPath,
		}
		if err := database.CreateRule(r.Context(), newRule); err != nil {
			WriteError(w, http.StatusInternalServerError, "rule_create_failed", err.Error())
			return
		}
		WriteJSON(w, http.StatusCreated, convertConfigRule(*newRule))
	default:
		MethodNotAllowedHandler(w, r)
	}
}

func (e *ConfigEditor) RuleByIDHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		MethodNotAllowedHandler(w, r)
		return
	}
	if err := e.ensureDatabase(r.Context()); err != nil {
		WriteError(w, http.StatusServiceUnavailable, "database_unavailable", err.Error())
		return
	}
	id, err := ruleIDFromPath(r.URL.Path)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if err := database.DeleteRule(r.Context(), id); err != nil {
		WriteError(w, http.StatusInternalServerError, "rule_delete_failed", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"message": "rule deleted"})
}

func (e *ConfigEditor) ApplyRuleHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPatch {
		MethodNotAllowedHandler(w, r)
		return
	}
	if err := e.ensureDatabase(r.Context()); err != nil {
		WriteError(w, http.StatusServiceUnavailable, "database_unavailable", err.Error())
		return
	}
	var req updateRuleModeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid_request", "failed to decode request body: "+err.Error())
		return
	}
	if req.ChatID == 0 {
		WriteError(w, http.StatusBadRequest, "invalid_request", "chat_id is required")
		return
	}
	if err := database.UpdateUserApplyRule(r.Context(), req.ChatID, req.ApplyRule); err != nil {
		WriteError(w, http.StatusInternalServerError, "rule_mode_update_failed", err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{
		"chat_id":    req.ChatID,
		"apply_rule": req.ApplyRule,
	})
}

func (e *ConfigEditor) reloadRuntimeConfig(ctx context.Context, before config.Config) (RuntimeApplyResult, error) {
	if err := config.Init(ctx, e.configPath); err != nil {
		return RuntimeApplyResult{}, fmt.Errorf("saved config but failed to reload runtime config: %w", err)
	}
	result := RuntimeApplyResult{}
	if e.autoOpenDatabase {
		if !database.Ready() {
			if err := database.Open(ctx); err != nil {
				result.Warnings = append(result.Warnings, "数据库打开失败: "+err.Error())
			}
		}
	}
	result.Warnings = append(result.Warnings, e.cleanupRemovedStorageReferences(ctx, removedRuntimeStorageNames(before, config.C()))...)
	applied := ApplyRuntimeConfig(ctx, e.configPath, before)
	applied.Warnings = append(result.Warnings, applied.Warnings...)
	return applied, nil
}

type storageReferenceChanges struct {
	Renamed map[string]string
	Removed []string
}

func (e *ConfigEditor) storageChanges(next *config.EditableConfig) storageReferenceChanges {
	current, err := config.LoadEditableConfig(e.configPath)
	if err != nil {
		return storageReferenceChanges{}
	}
	return diffEditableStorages(current.Config.Storages, next.Storages)
}

func (e *ConfigEditor) applyStorageReferenceChanges(ctx context.Context, changes storageReferenceChanges) []string {
	warnings := make([]string, 0)
	if database.Ready() {
		for oldName, newName := range changes.Renamed {
			if err := database.RenameStorageReferences(ctx, oldName, newName); err != nil {
				warnings = append(warnings, fmt.Sprintf("迁移存储 %s 到 %s 的用户关联失败: %v", oldName, newName, err))
			}
		}
	}
	warnings = append(warnings, e.cleanupRemovedStorageReferences(ctx, changes.Removed)...)
	return warnings
}

func (e *ConfigEditor) cleanupRemovedStorageReferences(ctx context.Context, names []string) []string {
	names = compactStrings(names)
	if len(names) == 0 || !database.Ready() {
		return nil
	}
	warnings := make([]string, 0)
	for _, name := range names {
		if err := database.ClearStorageReferences(ctx, name); err != nil {
			warnings = append(warnings, fmt.Sprintf("清理存储 %s 的用户关联失败: %v", name, err))
		}
	}
	return warnings
}

func diffEditableStorages(before, after []map[string]any) storageReferenceChanges {
	beforeNames := editableStorageNames(before)
	afterNames := editableStorageNames(after)
	changes := storageReferenceChanges{
		Renamed: make(map[string]string),
		Removed: make([]string, 0),
	}
	for i := range before {
		if i >= len(after) {
			continue
		}
		oldName := strings.TrimSpace(fmt.Sprint(before[i]["name"]))
		newName := strings.TrimSpace(fmt.Sprint(after[i]["name"]))
		if oldName == "" || newName == "" || oldName == newName {
			continue
		}
		if _, oldStillExists := afterNames[oldName]; oldStillExists {
			continue
		}
		if _, newExistedBefore := beforeNames[newName]; newExistedBefore {
			continue
		}
		changes.Renamed[oldName] = newName
	}
	for oldName := range beforeNames {
		if _, ok := afterNames[oldName]; ok {
			continue
		}
		if _, renamed := changes.Renamed[oldName]; renamed {
			continue
		}
		changes.Removed = append(changes.Removed, oldName)
	}
	return changes
}

func removedRuntimeStorageNames(before, after config.Config) []string {
	beforeNames := make(map[string]struct{}, len(before.Storages))
	afterNames := make(map[string]struct{}, len(after.Storages))
	for _, storage := range before.Storages {
		beforeNames[storage.GetName()] = struct{}{}
	}
	for _, storage := range after.Storages {
		afterNames[storage.GetName()] = struct{}{}
	}
	return missingStorageNames(beforeNames, afterNames)
}

func editableStorageNames(storages []map[string]any) map[string]struct{} {
	names := make(map[string]struct{}, len(storages))
	for _, storage := range storages {
		name := strings.TrimSpace(fmt.Sprint(storage["name"]))
		if name != "" {
			names[name] = struct{}{}
		}
	}
	return names
}

func missingStorageNames(before, after map[string]struct{}) []string {
	missing := make([]string, 0)
	for name := range before {
		if _, ok := after[name]; !ok {
			missing = append(missing, name)
		}
	}
	return missing
}

func (e *ConfigEditor) ensureDatabase(ctx context.Context) error {
	if database.Ready() {
		return nil
	}
	if !e.autoOpenDatabase {
		return fmt.Errorf("database is not initialized")
	}
	if err := config.Init(ctx, e.configPath); err != nil {
		return fmt.Errorf("load config before opening database: %w", err)
	}
	return database.Open(ctx)
}

func decodeEditableConfigRequest(r *http.Request) (*config.EditableConfig, error) {
	var raw json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to decode request body: %w", err)
	}
	var wrapped struct {
		Config *config.EditableConfig `json:"config"`
	}
	if err := json.Unmarshal(raw, &wrapped); err == nil && wrapped.Config != nil {
		return wrapped.Config, nil
	}
	var cfg config.EditableConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}
	return &cfg, nil
}

type configUserResponse struct {
	ID             uint                 `json:"id"`
	ChatID         int64                `json:"chat_id"`
	ApplyRule      bool                 `json:"apply_rule"`
	DefaultStorage string               `json:"default_storage"`
	DefaultDir     uint                 `json:"default_dir"`
	Rules          []configRuleResponse `json:"rules"`
}

type configRuleResponse struct {
	ID          uint   `json:"id"`
	Type        string `json:"type"`
	Data        string `json:"data"`
	StorageName string `json:"storage_name"`
	DirPath     string `json:"dir_path"`
}

type createRuleRequest struct {
	ChatID      int64  `json:"chat_id"`
	Type        string `json:"type"`
	Data        string `json:"data"`
	StorageName string `json:"storage_name"`
	DirPath     string `json:"dir_path"`
}

type updateRuleModeRequest struct {
	ChatID    int64 `json:"chat_id"`
	ApplyRule bool  `json:"apply_rule"`
}

func convertConfigUser(user database.User) configUserResponse {
	rules := make([]configRuleResponse, 0, len(user.Rules))
	for _, userRule := range user.Rules {
		rules = append(rules, convertConfigRule(userRule))
	}
	return configUserResponse{
		ID:             user.ID,
		ChatID:         user.ChatID,
		ApplyRule:      user.ApplyRule,
		DefaultStorage: user.DefaultStorage,
		DefaultDir:     user.DefaultDir,
		Rules:          rules,
	}
}

func convertConfigRule(userRule database.Rule) configRuleResponse {
	return configRuleResponse{
		ID:          userRule.ID,
		Type:        userRule.Type,
		Data:        userRule.Data,
		StorageName: userRule.StorageName,
		DirPath:     userRule.DirPath,
	}
}

func optionalChatID(r *http.Request) (int64, bool, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("chat_id"))
	if raw == "" {
		return 0, false, nil
	}
	chatID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, false, fmt.Errorf("invalid chat_id: %w", err)
	}
	return chatID, true, nil
}

func ruleIDFromPath(path string) (uint, error) {
	raw := strings.TrimPrefix(path, "/api/v1/config/rules/")
	if raw == "" || strings.Contains(raw, "/") {
		return 0, fmt.Errorf("rule id is required")
	}
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		return 0, fmt.Errorf("invalid rule id")
	}
	return uint(id), nil
}

func normalizeCreateRuleRequest(req createRuleRequest) (createRuleRequest, error) {
	req.Type = strings.ToUpper(strings.TrimSpace(req.Type))
	req.Data = strings.TrimSpace(req.Data)
	req.StorageName = strings.TrimSpace(req.StorageName)
	req.DirPath = strings.TrimSpace(req.DirPath)
	if req.ChatID == 0 {
		return req, fmt.Errorf("chat_id is required")
	}
	if req.Type == "" {
		return req, fmt.Errorf("type is required")
	}
	if req.Data == "" {
		return req, fmt.Errorf("data is required")
	}
	if req.StorageName == "" {
		return req, fmt.Errorf("storage_name is required")
	}
	if req.DirPath == "" {
		return req, fmt.Errorf("dir_path is required")
	}
	validType := false
	for _, value := range rule.Values() {
		if req.Type == value.String() {
			validType = true
			break
		}
	}
	if !validType {
		return req, fmt.Errorf("invalid rule type: %s", req.Type)
	}
	switch req.Type {
	case rule.FileNameRegex.String(), rule.MessageRegex.String():
		if _, err := regexp.Compile(req.Data); err != nil {
			return req, fmt.Errorf("invalid regex: %w", err)
		}
	case rule.IsAlbum.String():
		if _, err := strconv.ParseBool(req.Data); err != nil {
			return req, fmt.Errorf("IS-ALBUM data must be true or false")
		}
	}
	if req.StorageName != rule.RuleStorNameChosen && !configuredStorageNameExists(req.StorageName) {
		return req, fmt.Errorf("unknown storage_name: %s", req.StorageName)
	}
	return req, nil
}

func configuredStorageNames() []string {
	cfg, err := config.LoadEditableConfig(config.ConfigFileUsed())
	if err != nil {
		log.Warnf("failed to load configured storage names: %v", err)
		return nil
	}
	names := make([]string, 0, len(cfg.Config.Storages))
	for _, storage := range cfg.Config.Storages {
		name := strings.TrimSpace(fmt.Sprint(storage["name"]))
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

func configuredStorageNameExists(name string) bool {
	for _, configuredName := range configuredStorageNames() {
		if configuredName == name {
			return true
		}
	}
	return false
}
