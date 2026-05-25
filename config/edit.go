package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	storenum "github.com/krau/SaveAny-Bot/pkg/enums/storage"
	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/viper"
)

const (
	DefaultConfigFile = "config.toml"
	DefaultAPIPort    = 19191
)

type EditableConfig struct {
	Lang         string                 `toml:"lang" json:"lang"`
	Workers      int                    `toml:"workers" json:"workers"`
	Retry        int                    `toml:"retry" json:"retry"`
	Threads      int                    `toml:"threads" json:"threads"`
	Stream       bool                   `toml:"stream" json:"stream"`
	NoCleanCache bool                   `toml:"no_clean_cache" json:"no_clean_cache"`
	Proxy        string                 `toml:"proxy" json:"proxy"`
	Log          EditableLogConfig      `toml:"log" json:"log"`
	Telegram     EditableTelegramConfig `toml:"telegram" json:"telegram"`
	Aria2        EditableAria2Config    `toml:"aria2" json:"aria2"`
	API          EditableAPIConfig      `toml:"api" json:"api"`
	Cache        EditableCacheConfig    `toml:"cache" json:"cache"`
	Temp         EditableTempConfig     `toml:"temp" json:"temp"`
	DB           EditableDBConfig       `toml:"db" json:"db"`
	Parser       EditableParserConfig   `toml:"parser" json:"parser"`
	Hook         EditableHookConfig     `toml:"hook" json:"hook"`
	Storages     []map[string]any       `toml:"storages" json:"storages"`
	Users        []EditableUserConfig   `toml:"users" json:"users"`
}

type EditableLogConfig struct {
	Level string `toml:"level" json:"level"`
}

type EditableTelegramConfig struct {
	Token             string                      `toml:"token" json:"token"`
	AppID             int                         `toml:"app_id" json:"app_id"`
	AppHash           string                      `toml:"app_hash" json:"app_hash"`
	Proxy             EditableTelegramProxyConfig `toml:"proxy" json:"proxy"`
	RpcRetry          int                         `toml:"rpc_retry" json:"rpc_retry"`
	Userbot           EditableUserbotConfig       `toml:"userbot" json:"userbot"`
	MediaGroupTimeout int                         `toml:"media_group_timeout" json:"media_group_timeout"`
}

type EditableTelegramProxyConfig struct {
	Enable bool   `toml:"enable" json:"enable"`
	URL    string `toml:"url" json:"url"`
}

type EditableUserbotConfig struct {
	Enable  bool   `toml:"enable" json:"enable"`
	Session string `toml:"session" json:"session"`
}

type EditableAria2Config struct {
	Enable   bool   `toml:"enable" json:"enable"`
	Url      string `toml:"url" json:"url"`
	Secret   string `toml:"secret" json:"secret"`
	KeepFile bool   `toml:"keep_file" json:"keep_file"`
}

type EditableAPIConfig struct {
	Enable bool   `toml:"enable" json:"enable"`
	Host   string `toml:"host" json:"host"`
	Port   int    `toml:"port" json:"port"`
	Token  string `toml:"token" json:"token"`
}

type EditableCacheConfig struct {
	TTL         int64 `toml:"ttl" json:"ttl"`
	NumCounters int64 `toml:"num_counters" json:"num_counters"`
	MaxCost     int64 `toml:"max_cost" json:"max_cost"`
}

type EditableTempConfig struct {
	BasePath string `toml:"base_path" json:"base_path"`
}

type EditableDBConfig struct {
	Path    string `toml:"path" json:"path"`
	Session string `toml:"session" json:"session"`
}

type EditableParserConfig struct {
	PluginEnable bool                      `toml:"plugin_enable" json:"plugin_enable"`
	PluginDirs   []string                  `toml:"plugin_dirs" json:"plugin_dirs"`
	Proxy        string                    `toml:"proxy" json:"proxy"`
	ParserCfgs   map[string]map[string]any `toml:",inline" json:"parser_cfgs,omitempty"`
}

type EditableHookConfig struct {
	Exec EditableHookExecConfig `toml:"exec" json:"exec"`
}

type EditableHookExecConfig struct {
	TaskBeforeStart string `toml:"task_before_start" json:"task_before_start"`
	TaskSuccess     string `toml:"task_success" json:"task_success"`
	TaskFail        string `toml:"task_fail" json:"task_fail"`
	TaskCancel      string `toml:"task_cancel" json:"task_cancel"`
}

type EditableUserConfig struct {
	ID        int64    `toml:"id" json:"id"`
	Storages  []string `toml:"storages" json:"storages"`
	Blacklist bool     `toml:"blacklist" json:"blacklist"`
}

type EditableConfigFile struct {
	Path   string         `json:"path"`
	Exists bool           `json:"exists"`
	Config EditableConfig `json:"config"`
}

type StorageFieldSchema struct {
	Name        string   `json:"name"`
	Label       string   `json:"label"`
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Secret      bool     `json:"secret,omitempty"`
	Placeholder string   `json:"placeholder,omitempty"`
	Help        string   `json:"help,omitempty"`
	Options     []string `json:"options,omitempty"`
}

type StorageTypeSchema struct {
	Type   string               `json:"type"`
	Label  string               `json:"label"`
	Fields []StorageFieldSchema `json:"fields"`
}

func ResolveConfigFilePath(path string) string {
	if path != "" {
		return path
	}
	if used := viper.ConfigFileUsed(); used != "" {
		return used
	}
	return DefaultConfigFile
}

func ConfigFileUsed() string {
	return viper.ConfigFileUsed()
}

func DefaultEditableConfig() EditableConfig {
	return EditableConfig{
		Lang:    "zh-Hans",
		Workers: 4,
		Retry:   3,
		Threads: 4,
		Log: EditableLogConfig{
			Level: "debug",
		},
		Telegram: EditableTelegramConfig{
			AppID:    1025907,
			AppHash:  "452b0359b988148995f22ff0f4229750",
			RpcRetry: 5,
			Userbot: EditableUserbotConfig{
				Session: "data/usersession.db",
			},
		},
		Aria2: EditableAria2Config{
			Url: "http://localhost:6800/jsonrpc",
		},
		API: EditableAPIConfig{
			Host: "0.0.0.0",
			Port: DefaultAPIPort,
		},
		Cache: EditableCacheConfig{
			TTL:         86400,
			NumCounters: 100000,
			MaxCost:     1000000,
		},
		Temp: EditableTempConfig{
			BasePath: "cache/",
		},
		DB: EditableDBConfig{
			Path:    "data/saveany.db",
			Session: "data/session.db",
		},
		Parser: EditableParserConfig{
			PluginDirs: []string{"plugins"},
		},
		Storages: []map[string]any{
			{
				"name":      "local",
				"type":      storenum.Local.String(),
				"enable":    true,
				"base_path": "./downloads",
			},
		},
		Users: []EditableUserConfig{},
	}
}

func LoadEditableConfig(path string) (*EditableConfigFile, error) {
	path = ResolveConfigFilePath(path)
	cfg := DefaultEditableConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &EditableConfigFile{Path: path, Exists: false, Config: cfg}, nil
		}
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}
	if strings.TrimSpace(string(data)) == "" {
		return &EditableConfigFile{Path: path, Exists: true, Config: cfg}, nil
	}
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
	}
	NormalizeEditableConfig(&cfg)
	return &EditableConfigFile{Path: path, Exists: true, Config: cfg}, nil
}

func SaveEditableConfig(path string, cfg *EditableConfig) error {
	path = ResolveConfigFilePath(path)
	if isRemoteConfigPath(path) {
		return fmt.Errorf("remote config files are read-only in the web editor")
	}
	NormalizeEditableConfig(cfg)
	if err := ValidateEditableConfig(*cfg); err != nil {
		return err
	}
	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil && filepath.Dir(path) != "." {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	mode := os.FileMode(0644)
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode().Perm()
		if old, err := os.ReadFile(path); err == nil {
			_ = os.WriteFile(path+".bak", old, mode)
		}
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return fmt.Errorf("failed to write temporary config file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("failed to replace config file: %w", err)
	}
	return nil
}

func NormalizeEditableConfig(cfg *EditableConfig) {
	defaults := DefaultEditableConfig()
	if cfg.Lang == "" {
		cfg.Lang = defaults.Lang
	}
	if cfg.Workers < 1 {
		cfg.Workers = defaults.Workers
	}
	if cfg.Retry < 1 {
		cfg.Retry = defaults.Retry
	}
	if cfg.Threads < 1 {
		cfg.Threads = defaults.Threads
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = defaults.Log.Level
	}
	if cfg.Telegram.AppID == 0 {
		cfg.Telegram.AppID = defaults.Telegram.AppID
	}
	if cfg.Telegram.AppHash == "" {
		cfg.Telegram.AppHash = defaults.Telegram.AppHash
	}
	if cfg.Telegram.RpcRetry == 0 {
		cfg.Telegram.RpcRetry = defaults.Telegram.RpcRetry
	}
	if cfg.Telegram.Userbot.Session == "" {
		cfg.Telegram.Userbot.Session = defaults.Telegram.Userbot.Session
	}
	if cfg.Aria2.Url == "" {
		cfg.Aria2.Url = defaults.Aria2.Url
	}
	if cfg.API.Host == "" {
		cfg.API.Host = defaults.API.Host
	}
	if cfg.API.Port == 0 {
		cfg.API.Port = defaults.API.Port
	}
	if cfg.Cache.TTL == 0 {
		cfg.Cache.TTL = defaults.Cache.TTL
	}
	if cfg.Cache.NumCounters == 0 {
		cfg.Cache.NumCounters = defaults.Cache.NumCounters
	}
	if cfg.Cache.MaxCost == 0 {
		cfg.Cache.MaxCost = defaults.Cache.MaxCost
	}
	if cfg.Temp.BasePath == "" {
		cfg.Temp.BasePath = defaults.Temp.BasePath
	}
	if cfg.DB.Path == "" {
		cfg.DB.Path = defaults.DB.Path
	}
	if cfg.DB.Session == "" {
		cfg.DB.Session = defaults.DB.Session
	}
	if cfg.Parser.PluginDirs == nil {
		cfg.Parser.PluginDirs = defaults.Parser.PluginDirs
	}
	if cfg.Storages == nil {
		cfg.Storages = []map[string]any{}
	}
	if cfg.Users == nil {
		cfg.Users = []EditableUserConfig{}
	}
	for i := range cfg.Storages {
		normalizeStorageMap(cfg.Storages[i])
		fillDefaultStoragePath(cfg.Storages[i])
	}
	storageNames := editableStorageNameSet(cfg.Storages)
	for i := range cfg.Users {
		if cfg.Users[i].Storages == nil {
			cfg.Users[i].Storages = []string{}
		}
		cfg.Users[i].Storages = filterKnownStorageNames(cfg.Users[i].Storages, storageNames)
	}
}

func ValidateEditableConfig(cfg EditableConfig) error {
	if cfg.Workers < 1 {
		return fmt.Errorf("workers must be greater than 0")
	}
	if cfg.Retry < 1 {
		return fmt.Errorf("retry must be greater than 0")
	}
	if cfg.Threads < 1 {
		return fmt.Errorf("threads must be greater than 0")
	}
	if !slices.Contains([]string{"trace", "debug", "info", "warn", "error", "fatal"}, strings.ToLower(cfg.Log.Level)) {
		return fmt.Errorf("invalid log level: %s", cfg.Log.Level)
	}
	if err := validateProxyURL("proxy", cfg.Proxy, false); err != nil {
		return err
	}
	if err := validateProxyURL("telegram.proxy.url", cfg.Telegram.Proxy.URL, cfg.Telegram.Proxy.Enable); err != nil {
		return err
	}
	if err := validateProxyURL("parser.proxy", cfg.Parser.Proxy, false); err != nil {
		return err
	}
	if cfg.API.Port < 1 || cfg.API.Port > 65535 {
		return fmt.Errorf("api.port must be between 1 and 65535")
	}
	if cfg.Cache.TTL < 0 || cfg.Cache.NumCounters < 0 || cfg.Cache.MaxCost < 0 {
		return fmt.Errorf("cache values must not be negative")
	}
	if cfg.DB.Path == "" {
		return fmt.Errorf("db.path is required")
	}
	if cfg.DB.Session == "" {
		return fmt.Errorf("db.session is required")
	}
	storageNames := make(map[string]struct{}, len(cfg.Storages))
	for i, storage := range cfg.Storages {
		if err := validateEditableStorage(i, storage); err != nil {
			return err
		}
		name := getStringValue(storage, "name")
		if _, ok := storageNames[name]; ok {
			return fmt.Errorf("duplicate storage name: %s", name)
		}
		storageNames[name] = struct{}{}
	}
	userIDs := make(map[int64]struct{}, len(cfg.Users))
	for _, user := range cfg.Users {
		if user.ID == 0 {
			return fmt.Errorf("user id is required")
		}
		if _, ok := userIDs[user.ID]; ok {
			return fmt.Errorf("duplicate user id: %d", user.ID)
		}
		userIDs[user.ID] = struct{}{}
		for _, storageName := range user.Storages {
			if _, ok := storageNames[storageName]; !ok {
				return fmt.Errorf("user %d references unknown storage %s", user.ID, storageName)
			}
		}
	}
	return nil
}

func StorageSchemas() []StorageTypeSchema {
	return []StorageTypeSchema{
		{
			Type:  storenum.Local.String(),
			Label: "Local",
			Fields: []StorageFieldSchema{
				{Name: "base_path", Label: "根路径", Type: "string", Required: true, Placeholder: "./downloads"},
			},
		},
		{
			Type:  storenum.Webdav.String(),
			Label: "WebDAV",
			Fields: []StorageFieldSchema{
				{Name: "url", Label: "URL", Type: "url", Required: true, Placeholder: "https://example.com/dav"},
				{Name: "username", Label: "用户名", Type: "string", Required: true},
				{Name: "password", Label: "密码", Type: "password", Required: true, Secret: true},
				{Name: "base_path", Label: "根路径", Type: "string", Required: true, Placeholder: "/telegram"},
			},
		},
		{
			Type:  storenum.Alist.String(),
			Label: "AList",
			Fields: []StorageFieldSchema{
				{Name: "url", Label: "URL", Type: "url", Required: true, Placeholder: "https://alist.example.com"},
				{Name: "username", Label: "用户名", Type: "string"},
				{Name: "password", Label: "密码", Type: "password", Secret: true},
				{Name: "token", Label: "Token", Type: "password", Secret: true, Help: "Token 与用户名密码二选一"},
				{Name: "base_path", Label: "根路径", Type: "string", Required: true, Placeholder: "/telegram"},
				{Name: "token_exp", Label: "Token 过期时间", Type: "int"},
			},
		},
		{
			Type:   storenum.Minio.String(),
			Label:  "MinIO",
			Fields: objectStorageFields(false),
		},
		{
			Type:   storenum.S3.String(),
			Label:  "S3",
			Fields: objectStorageFields(true),
		},
		{
			Type:  storenum.Telegram.String(),
			Label: "Telegram",
			Fields: []StorageFieldSchema{
				{Name: "chat_id", Label: "Chat ID", Type: "int", Required: true, Placeholder: "-1001234567890"},
				{Name: "force_file", Label: "强制文件模式", Type: "bool"},
				{Name: "rate_limit", Label: "速率限制", Type: "int"},
				{Name: "rate_burst", Label: "突发限制", Type: "int"},
				{Name: "skip_large", Label: "跳过超大文件", Type: "bool"},
				{Name: "split_size_mb", Label: "分卷大小 MB", Type: "int"},
			},
		},
		{
			Type:  storenum.Rclone.String(),
			Label: "Rclone",
			Fields: []StorageFieldSchema{
				{Name: "remote", Label: "Remote", Type: "string", Required: true, Placeholder: "remote:"},
				{Name: "base_path", Label: "根路径", Type: "string", Placeholder: "/telegram"},
				{Name: "config_path", Label: "配置文件路径", Type: "string"},
				{Name: "flags", Label: "额外参数", Type: "string_list", Placeholder: "--transfers=4"},
			},
		},
	}
}

func StorageTypeNames() []string {
	return storenum.StorageTypeNames()
}

func objectStorageFields(includeS3Only bool) []StorageFieldSchema {
	fields := []StorageFieldSchema{
		{Name: "endpoint", Label: "Endpoint", Type: "string", Required: true, Placeholder: "s3.amazonaws.com"},
		{Name: "access_key_id", Label: "Access Key ID", Type: "string", Required: true},
		{Name: "secret_access_key", Label: "Secret Access Key", Type: "password", Required: true, Secret: true},
		{Name: "bucket_name", Label: "Bucket", Type: "string", Required: true},
		{Name: "use_ssl", Label: "Use SSL", Type: "bool"},
		{Name: "base_path", Label: "根路径", Type: "string", Required: true, Placeholder: "telegram"},
	}
	if includeS3Only {
		fields = append(fields,
			StorageFieldSchema{Name: "region", Label: "Region", Type: "string", Placeholder: "ap-east-1"},
			StorageFieldSchema{Name: "virtual_host", Label: "Virtual Host", Type: "bool"},
		)
	}
	return fields
}

func validateEditableStorage(index int, storage map[string]any) error {
	name := getStringValue(storage, "name")
	if name == "" {
		return fmt.Errorf("storages[%d].name is required", index)
	}
	storageType := getStringValue(storage, "type")
	if storageType == "" {
		return fmt.Errorf("storage %s type is required", name)
	}
	parsedType, err := storenum.ParseStorageType(storageType)
	if err != nil {
		return fmt.Errorf("invalid storage type %s for %s: %w", storageType, name, err)
	}
	if !getBoolValue(storage, "enable") {
		return nil
	}
	required := func(key string) error {
		if strings.TrimSpace(getStringValue(storage, key)) == "" {
			return fmt.Errorf("%s is required for %s storage %s", key, parsedType, name)
		}
		return nil
	}
	switch parsedType {
	case storenum.Local:
		return required("base_path")
	case storenum.Webdav:
		for _, key := range []string{"url", "username", "password", "base_path"} {
			if err := required(key); err != nil {
				return err
			}
		}
	case storenum.Alist:
		if err := required("url"); err != nil {
			return err
		}
		if getStringValue(storage, "token") == "" && (getStringValue(storage, "username") == "" || getStringValue(storage, "password") == "") {
			return fmt.Errorf("username and password or token is required for alist storage %s", name)
		}
		return required("base_path")
	case storenum.Minio, storenum.S3:
		for _, key := range []string{"endpoint", "access_key_id", "secret_access_key", "bucket_name", "base_path"} {
			if err := required(key); err != nil {
				return err
			}
		}
	case storenum.Telegram:
		if getInt64Value(storage, "chat_id") == 0 {
			return fmt.Errorf("chat_id is required for telegram storage %s", name)
		}
		if getInt64Value(storage, "rate_limit") < 0 || getInt64Value(storage, "rate_burst") < 0 {
			return fmt.Errorf("rate_limit and rate_burst must not be negative for telegram storage %s", name)
		}
	case storenum.Rclone:
		return required("remote")
	}
	return nil
}

func fillDefaultStoragePath(storage map[string]any) {
	defaultPath, ok := defaultStorageBasePath(getStringValue(storage, "type"))
	if !ok {
		return
	}
	if strings.TrimSpace(getStringValue(storage, "base_path")) == "" {
		storage["base_path"] = defaultPath
	}
}

func defaultStorageBasePath(storageType string) (string, bool) {
	switch storageType {
	case storenum.Local.String():
		return "./downloads", true
	case storenum.Webdav.String(), storenum.Alist.String(), storenum.Rclone.String():
		return "/telegram", true
	case storenum.Minio.String(), storenum.S3.String():
		return "telegram", true
	default:
		return "", false
	}
}

func editableStorageNameSet(storages []map[string]any) map[string]struct{} {
	names := make(map[string]struct{}, len(storages))
	for _, storage := range storages {
		name := getStringValue(storage, "name")
		if name != "" {
			names[name] = struct{}{}
		}
	}
	return names
}

func validateProxyURL(field, value string, required bool) error {
	value = strings.TrimSpace(value)
	if value == "" {
		if required {
			return fmt.Errorf("%s is required", field)
		}
		return nil
	}
	u, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("invalid %s: %w", field, err)
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https", "socks5", "socks5h":
		return nil
	default:
		return fmt.Errorf("%s must use http, https, socks5, or socks5h", field)
	}
}

func isRemoteConfigPath(path string) bool {
	return strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://")
}

func normalizeStorageMap(storage map[string]any) {
	for _, schema := range StorageSchemas() {
		if getStringValue(storage, "type") != schema.Type {
			continue
		}
		for _, field := range schema.Fields {
			switch field.Type {
			case "int":
				if value, ok := maybeInt64(storage[field.Name]); ok {
					storage[field.Name] = value
				}
			case "bool":
				if value, ok := maybeBool(storage[field.Name]); ok {
					storage[field.Name] = value
				}
			case "string_list":
				if value, ok := maybeStringList(storage[field.Name]); ok {
					storage[field.Name] = value
				}
			default:
				if value, ok := storage[field.Name]; ok {
					storage[field.Name] = strings.TrimSpace(fmt.Sprint(value))
				}
			}
		}
		break
	}
	if value, ok := storage["enable"]; ok {
		if b, ok := maybeBool(value); ok {
			storage["enable"] = b
		}
	}
}

func getStringValue(values map[string]any, key string) string {
	value, ok := values[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func getBoolValue(values map[string]any, key string) bool {
	value, ok := values[key]
	if !ok {
		return false
	}
	b, _ := maybeBool(value)
	return b
}

func getInt64Value(values map[string]any, key string) int64 {
	value, ok := values[key]
	if !ok {
		return 0
	}
	i, _ := maybeInt64(value)
	return i
}

func maybeBool(value any) (bool, bool) {
	switch v := value.(type) {
	case bool:
		return v, true
	case string:
		b, err := strconv.ParseBool(strings.TrimSpace(v))
		return b, err == nil
	default:
		return false, false
	}
}

func maybeInt64(value any) (int64, bool) {
	switch v := value.(type) {
	case int:
		return int64(v), true
	case int8:
		return int64(v), true
	case int16:
		return int64(v), true
	case int32:
		return int64(v), true
	case int64:
		return v, true
	case uint:
		return int64(v), true
	case uint8:
		return int64(v), true
	case uint16:
		return int64(v), true
	case uint32:
		return int64(v), true
	case uint64:
		if v > uint64(^uint64(0)>>1) {
			return 0, false
		}
		return int64(v), true
	case float32:
		return int64(v), float32(int64(v)) == v
	case float64:
		return int64(v), float64(int64(v)) == v
	case string:
		i, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		return i, err == nil
	default:
		return 0, false
	}
}

func maybeStringList(value any) ([]string, bool) {
	switch v := value.(type) {
	case []string:
		return v, true
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			out = append(out, strings.TrimSpace(fmt.Sprint(item)))
		}
		return out, true
	case string:
		if strings.TrimSpace(v) == "" {
			return []string{}, true
		}
		parts := regexp.MustCompile(`[\n,]+`).Split(v, -1)
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				out = append(out, part)
			}
		}
		return out, true
	default:
		return nil, false
	}
}
