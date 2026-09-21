package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config 全局配置结构体
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Log      LogConfig      `yaml:"log"`
}

type ServerConfig struct {
	Addr         string `yaml:"addr"`
	ReadTimeout  int    `yaml:"read_timeout"`
	WriteTimeout int    `yaml:"write_timeout"`
}

type DatabaseConfig struct {
	Driver string `yaml:"driver"`
	DSN    string `yaml:"dsn"`
}

type LogConfig struct {
	Level    string `yaml:"level"`    // debug / info / warn / error / fatal（默认 info）
	Format   string `yaml:"format"`   // text / json，默认 text
	Console  bool   `yaml:"console"`  // 是否输出到控制台
	Dir      string `yaml:"dir"`      // 日志目录，空则不写文件
	Filename string `yaml:"filename"` // 日志文件名（不含扩展名），空则取程序名
	MaxAge   int    `yaml:"max_age"`  // 日志保留天数，<=0 不清理
	Caller   bool   `yaml:"caller"`   // 是否输出调用位置（单行 file:line）

	// parent_span_ids 是否输出中间 span 链（默认关闭，开启有聚合开销）
	ParentSpanIDs bool `yaml:"parent_span_ids"`

	// resource 元信息（写入日志的 resource 对象）
	Service string `yaml:"service"` // service.name
	Version string `yaml:"version"` // service.version
	Env     string `yaml:"env"`     // deployment.environment
	Host    string `yaml:"host"`    // host.name（空则取本机主机名）
}

// defaultConfig 默认配置
func defaultConfig() *Config {
	return &Config{
		Server:   ServerConfig{Addr: ":8080", ReadTimeout: 30, WriteTimeout: 30},
		Database: DatabaseConfig{Driver: "sqlite", DSN: "app.db"},
		Log:      LogConfig{Level: "info"},
	}
}

// Load 加载配置：默认值（代码兜底） < 配置文件 < 环境变量
// 入口文件可通过 include 引用其他文件，重复 key 直接 panic
func Load(configFile string) (*Config, error) {
	// 1. 解析所有文件，合并为原始 map
	merged := make(map[string]any)
	if configFile != "" {
		if err := loadAndMerge(configFile, merged); err != nil {
			return nil, err
		}
	}

	// 2. 将合并后的 map 反序列化为 Config 结构体
	cfg := defaultConfig()
	if err := mapToConfig(merged, cfg); err != nil {
		return nil, err
	}

	// 3. 环境变量覆盖
	overrideFromEnv(cfg)

	return cfg, nil
}

// loadAndMerge 加载一个 yaml 文件，处理 include，合并到 merged map
// 遇到重复 key 直接 panic
func loadAndMerge(path string, merged map[string]any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read %s: %w", path, err)
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}

	// 处理 include 指令，先递归加载子文件
	if includes, ok := raw["include"]; ok {
		delete(raw, "include") // include 不参与业务配置

		dir := filepath.Dir(path)
		for _, inc := range includes.([]any) {
			incPath := filepath.Join(dir, inc.(string))
			if err := loadAndMerge(incPath, merged); err != nil {
				return err
			}
		}
	}

	// 合并当前文件的 key，遇到重复直接 panic
	mergeMap(merged, raw, path)
	return nil
}

// mergeMap 将 src 合并入 dst，遇到重复 key 直接 panic
func mergeMap(dst, src map[string]any, srcFile string) {
	for k, v := range src {
		if _, exists := dst[k]; exists {
			panic(fmt.Sprintf("config: duplicate key %q found in %s", k, srcFile))
		}
		dst[k] = v
	}
}

// mapToConfig 将原始 map 序列化回 Config 结构体（借助 yaml 中转）
func mapToConfig(m map[string]any, cfg *Config) error {
	data, err := yaml.Marshal(m)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, cfg)
}

// overrideFromEnv 用环境变量覆盖配置
// readFile 读取文件内容
func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func overrideFromEnv(cfg *Config) {
	if v := os.Getenv("APP_ADDR"); v != "" {
		cfg.Server.Addr = v
	}
	if v := os.Getenv("APP_READ_TIMEOUT"); v != "" {
		fmt.Sscanf(v, "%d", &cfg.Server.ReadTimeout)
	}
	if v := os.Getenv("APP_WRITE_TIMEOUT"); v != "" {
		fmt.Sscanf(v, "%d", &cfg.Server.WriteTimeout)
	}
	if v := os.Getenv("APP_DB_DRIVER"); v != "" {
		cfg.Database.Driver = v
	}
	if v := os.Getenv("APP_DB_DSN"); v != "" {
		cfg.Database.DSN = v
	}
	if v := os.Getenv("APP_LOG_LEVEL"); v != "" {
		cfg.Log.Level = v
	}
	if v := os.Getenv("APP_LOG_DIR"); v != "" {
		cfg.Log.Dir = v
	}
	if v := os.Getenv("APP_LOG_FILENAME"); v != "" {
		cfg.Log.Filename = v
	}
	if v := os.Getenv("APP_LOG_FORMAT"); v != "" {
		cfg.Log.Format = v
	}
	if v := os.Getenv("APP_LOG_CONSOLE"); v == "true" {
		cfg.Log.Console = true
	}
	if v := os.Getenv("APP_LOG_MAX_AGE"); v != "" {
		fmt.Sscanf(v, "%d", &cfg.Log.MaxAge)
	}
	if v := os.Getenv("APP_LOG_CALLER"); v == "true" {
		cfg.Log.Caller = true
	}
	if v := os.Getenv("APP_LOG_PARENT_SPAN_IDS"); v == "true" {
		cfg.Log.ParentSpanIDs = true
	}
	if v := os.Getenv("APP_LOG_SERVICE"); v != "" {
		cfg.Log.Service = v
	}
	if v := os.Getenv("APP_LOG_VERSION"); v != "" {
		cfg.Log.Version = v
	}
	if v := os.Getenv("APP_LOG_ENV"); v != "" {
		cfg.Log.Env = v
	}
	if v := os.Getenv("APP_LOG_HOST"); v != "" {
		cfg.Log.Host = v
	}
}
