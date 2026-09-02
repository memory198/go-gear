package config

import (
	"os"
	"path/filepath"
	"testing"
)

// ============================================================
// 辅助函数
// ============================================================

// writeFile 在 dir 目录下创建指定文件名的文件，写入 content，测试失败时终止
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("创建测试文件 %s 失败: %v", path, err)
	}
	return path
}

// ============================================================
// Load 基础加载测试
// ============================================================

// TestLoad_BasicFields 正常加载包含 server/database/log 三个段的完整配置文件，
// 验证各字段均被正确解析到对应的结构体字段。
func TestLoad_BasicFields(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "config.yaml", `
server:
  addr: ":9090"
  read_timeout: 10
  write_timeout: 20
database:
  driver: "postgres"
  dsn: "host=localhost"
log:
  level: "debug"
  format: "json"
  console: true
  caller: true
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() 返回错误: %v", err)
	}

	// 验证 server 字段
	if cfg.Server.Addr != ":9090" {
		t.Errorf("Server.Addr: 期望 \":9090\"，实际 %q", cfg.Server.Addr)
	}
	if cfg.Server.ReadTimeout != 10 {
		t.Errorf("Server.ReadTimeout: 期望 10，实际 %d", cfg.Server.ReadTimeout)
	}
	if cfg.Server.WriteTimeout != 20 {
		t.Errorf("Server.WriteTimeout: 期望 20，实际 %d", cfg.Server.WriteTimeout)
	}

	// 验证 database 字段
	if cfg.Database.Driver != "postgres" {
		t.Errorf("Database.Driver: 期望 \"postgres\"，实际 %q", cfg.Database.Driver)
	}
	if cfg.Database.DSN != "host=localhost" {
		t.Errorf("Database.DSN: 期望 \"host=localhost\"，实际 %q", cfg.Database.DSN)
	}

	// 验证 log 字段
	if cfg.Log.Level != "debug" {
		t.Errorf("Log.Level: 期望 \"debug\"，实际 %q", cfg.Log.Level)
	}
	if cfg.Log.Format != "json" {
		t.Errorf("Log.Format: 期望 \"json\"，实际 %q", cfg.Log.Format)
	}
	if !cfg.Log.Console {
		t.Error("Log.Console: 期望 true，实际 false")
	}
	if !cfg.Log.Caller {
		t.Error("Log.Caller: 期望 true，实际 false")
	}
}

// TestLoad_DefaultValues 传入空字符串时不加载任何文件，
// 验证返回的配置包含所有预设默认值（由 defaultConfig() 定义）。
func TestLoad_DefaultValues(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load(\"\") 返回错误: %v", err)
	}

	if cfg.Server.Addr != ":8080" {
		t.Errorf("默认 Server.Addr: 期望 \":8080\"，实际 %q", cfg.Server.Addr)
	}
	if cfg.Server.ReadTimeout != 30 {
		t.Errorf("默认 Server.ReadTimeout: 期望 30，实际 %d", cfg.Server.ReadTimeout)
	}
	if cfg.Server.WriteTimeout != 30 {
		t.Errorf("默认 Server.WriteTimeout: 期望 30，实际 %d", cfg.Server.WriteTimeout)
	}
	if cfg.Database.Driver != "sqlite" {
		t.Errorf("默认 Database.Driver: 期望 \"sqlite\"，实际 %q", cfg.Database.Driver)
	}
	if cfg.Database.DSN != "app.db" {
		t.Errorf("默认 Database.DSN: 期望 \"app.db\"，实际 %q", cfg.Database.DSN)
	}
	if cfg.Log.Level != "info" {
		t.Errorf("默认 Log.Level: 期望 \"info\"，实际 %q", cfg.Log.Level)
	}
}

// TestLoad_PartialConfig 只配置部分字段时，未配置的字段应保留默认值，
// 而不是被零值覆盖。
func TestLoad_PartialConfig(t *testing.T) {
	dir := t.TempDir()
	// 只覆盖 server.addr，其余字段不写
	path := writeFile(t, dir, "config.yaml", `
server:
  addr: ":7777"
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() 返回错误: %v", err)
	}

	if cfg.Server.Addr != ":7777" {
		t.Errorf("Server.Addr: 期望 \":7777\"，实际 %q", cfg.Server.Addr)
	}
	// ReadTimeout 未配置，应保留默认值 30
	if cfg.Server.ReadTimeout != 30 {
		t.Errorf("未配置的 Server.ReadTimeout 应为默认值 30，实际 %d", cfg.Server.ReadTimeout)
	}
	// Database 未配置，应全部保留默认值
	if cfg.Database.Driver != "sqlite" {
		t.Errorf("未配置的 Database.Driver 应为默认值 \"sqlite\"，实际 %q", cfg.Database.Driver)
	}
}

// TestLoad_FileNotFound 配置文件路径不存在时，
// loadAndMerge 内部通过 os.IsNotExist 静默忽略，
// Load() 应正常返回，不报错，并使用默认值。
func TestLoad_FileNotFound(t *testing.T) {
	cfg, err := Load("/tmp/nonexistent_gear_config_test.yaml")
	if err != nil {
		t.Fatalf("文件不存在时期望不报错，实际: %v", err)
	}
	// 应退化为默认配置
	if cfg.Server.Addr != ":8080" {
		t.Errorf("文件不存在时应使用默认 addr \":8080\"，实际 %q", cfg.Server.Addr)
	}
}

// TestLoad_MalformedYAML 配置文件内容不是合法 YAML 时，
// Load() 应返回解析错误，而不是静默失败。
func TestLoad_MalformedYAML(t *testing.T) {
	dir := t.TempDir()
	// 写入非法 YAML（缩进混乱）
	path := writeFile(t, dir, "bad.yaml", `
server:
  addr: ":8080"
 broken_indent: bad
`)

	_, err := Load(path)
	if err == nil {
		t.Error("期望非法 YAML 返回错误，实际返回 nil")
	}
}

// TestLoad_EmptyFile 空文件不包含任何字段，应等价于无配置，退化为全默认值。
func TestLoad_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "empty.yaml", "")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("空配置文件不应报错，实际: %v", err)
	}
	if cfg.Server.Addr != ":8080" {
		t.Errorf("空文件时应使用默认 addr，实际 %q", cfg.Server.Addr)
	}
}

// ============================================================
// include 指令测试
// ============================================================

// TestLoad_Include 主配置通过 include 引用子文件，
// 合并后所有字段（来自主文件和子文件）均可正确读取。
func TestLoad_Include(t *testing.T) {
	dir := t.TempDir()

	// 子配置文件：只包含 database 段
	writeFile(t, dir, "db.yaml", `
database:
  driver: "mysql"
  dsn: "root:pass@tcp(127.0.0.1:3306)/app"
`)

	// 主配置文件：包含 server 段，并通过 include 引入 db.yaml
	mainPath := writeFile(t, dir, "main.yaml", `
include:
  - db.yaml
server:
  addr: ":8888"
`)

	cfg, err := Load(mainPath)
	if err != nil {
		t.Fatalf("Load() 返回错误: %v", err)
	}

	// 主文件中的字段
	if cfg.Server.Addr != ":8888" {
		t.Errorf("Server.Addr: 期望 \":8888\"，实际 %q", cfg.Server.Addr)
	}
	// 子文件中的字段
	if cfg.Database.Driver != "mysql" {
		t.Errorf("Database.Driver: 期望 \"mysql\"，实际 %q", cfg.Database.Driver)
	}
	if cfg.Database.DSN != "root:pass@tcp(127.0.0.1:3306)/app" {
		t.Errorf("Database.DSN 不匹配，实际 %q", cfg.Database.DSN)
	}
}

// TestLoad_Include_NestedInclude 验证 include 的递归能力：
// 主文件 include 中间文件，中间文件再 include 叶子文件，三层均可合并。
func TestLoad_Include_NestedInclude(t *testing.T) {
	dir := t.TempDir()

	// 叶子文件：log 段
	writeFile(t, dir, "log.yaml", `
log:
  level: "warn"
`)
	// 中间文件：database 段 + include log.yaml
	writeFile(t, dir, "db.yaml", `
include:
  - log.yaml
database:
  driver: "postgres"
  dsn: "host=db"
`)
	// 主文件：server 段 + include db.yaml
	mainPath := writeFile(t, dir, "main.yaml", `
include:
  - db.yaml
server:
  addr: ":5000"
`)

	cfg, err := Load(mainPath)
	if err != nil {
		t.Fatalf("Load() 返回错误: %v", err)
	}
	if cfg.Server.Addr != ":5000" {
		t.Errorf("Server.Addr 期望 \":5000\"，实际 %q", cfg.Server.Addr)
	}
	if cfg.Database.Driver != "postgres" {
		t.Errorf("Database.Driver 期望 \"postgres\"，实际 %q", cfg.Database.Driver)
	}
	if cfg.Log.Level != "warn" {
		t.Errorf("Log.Level 期望 \"warn\"，实际 %q", cfg.Log.Level)
	}
}

// TestLoad_Include_DuplicateKey 主文件与 include 的子文件中存在相同顶级 key 时，
// mergeMap 应直接 panic（这是有意为之的 fail-fast 设计，防止静默覆盖配置）。
func TestLoad_Include_DuplicateKey(t *testing.T) {
	dir := t.TempDir()

	// 子文件也定义了 server 段
	writeFile(t, dir, "extra.yaml", `
server:
  addr: ":9999"
`)
	// 主文件同样定义了 server 段，与子文件冲突
	mainPath := writeFile(t, dir, "main.yaml", `
include:
  - extra.yaml
server:
  addr: ":8080"
`)

	// 期望触发 panic
	defer func() {
		if r := recover(); r == nil {
			t.Error("重复 key 应触发 panic，但没有")
		}
	}()
	_, _ = Load(mainPath)
}

// ============================================================
// 环境变量覆盖测试
// ============================================================

// TestLoad_EnvOverride_Addr 设置 APP_ADDR 环境变量后，
// 应覆盖配置文件中的 server.addr。
func TestLoad_EnvOverride_Addr(t *testing.T) {
	t.Setenv("APP_ADDR", ":1234")
	// t.Setenv 在测试结束时自动恢复原始环境变量，无需手动 Unsetenv

	dir := t.TempDir()
	path := writeFile(t, dir, "config.yaml", `
server:
  addr: ":8080"
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() 返回错误: %v", err)
	}
	if cfg.Server.Addr != ":1234" {
		t.Errorf("APP_ADDR 应覆盖 server.addr，期望 \":1234\"，实际 %q", cfg.Server.Addr)
	}
}

// TestLoad_EnvOverride_Database 设置 APP_DB_DRIVER 和 APP_DB_DSN，
// 应覆盖配置文件中的 database 段。
func TestLoad_EnvOverride_Database(t *testing.T) {
	t.Setenv("APP_DB_DRIVER", "sqlite3")
	t.Setenv("APP_DB_DSN", ":memory:")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() 返回错误: %v", err)
	}
	if cfg.Database.Driver != "sqlite3" {
		t.Errorf("APP_DB_DRIVER 应覆盖 driver，期望 \"sqlite3\"，实际 %q", cfg.Database.Driver)
	}
	if cfg.Database.DSN != ":memory:" {
		t.Errorf("APP_DB_DSN 应覆盖 dsn，期望 \":memory:\"，实际 %q", cfg.Database.DSN)
	}
}

// TestLoad_EnvOverride_Timeout APP_READ_TIMEOUT 和 APP_WRITE_TIMEOUT 使用 fmt.Sscanf 解析，
// 验证数字字符串能正确覆盖整型字段。
func TestLoad_EnvOverride_Timeout(t *testing.T) {
	t.Setenv("APP_READ_TIMEOUT", "60")
	t.Setenv("APP_WRITE_TIMEOUT", "120")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() 返回错误: %v", err)
	}
	if cfg.Server.ReadTimeout != 60 {
		t.Errorf("APP_READ_TIMEOUT 应覆盖 read_timeout，期望 60，实际 %d", cfg.Server.ReadTimeout)
	}
	if cfg.Server.WriteTimeout != 120 {
		t.Errorf("APP_WRITE_TIMEOUT 应覆盖 write_timeout，期望 120，实际 %d", cfg.Server.WriteTimeout)
	}
}

// TestLoad_EnvOverride_Log 验证所有 log 相关环境变量均能生效。
func TestLoad_EnvOverride_Log(t *testing.T) {
	t.Setenv("APP_LOG_LEVEL", "error")
	t.Setenv("APP_LOG_FORMAT", "json")
	t.Setenv("APP_LOG_CONSOLE", "true")
	t.Setenv("APP_LOG_DIR", "/var/log/app")
	t.Setenv("APP_LOG_FILENAME", "app")
	t.Setenv("APP_LOG_MAX_AGE", "7")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() 返回错误: %v", err)
	}
	if cfg.Log.Level != "error" {
		t.Errorf("Log.Level 期望 \"error\"，实际 %q", cfg.Log.Level)
	}
	if cfg.Log.Format != "json" {
		t.Errorf("Log.Format 期望 \"json\"，实际 %q", cfg.Log.Format)
	}
	if !cfg.Log.Console {
		t.Error("Log.Console 期望 true")
	}
	if cfg.Log.Dir != "/var/log/app" {
		t.Errorf("Log.Dir 期望 \"/var/log/app\"，实际 %q", cfg.Log.Dir)
	}
	if cfg.Log.Filename != "app" {
		t.Errorf("Log.Filename 期望 \"app\"，实际 %q", cfg.Log.Filename)
	}
	if cfg.Log.MaxAge != 7 {
		t.Errorf("Log.MaxAge 期望 7，实际 %d", cfg.Log.MaxAge)
	}
}

// TestLoad_EnvOverride_Partial 只设置部分环境变量时，
// 其他字段应保持文件中的值不变，验证覆盖是精确的而非全局替换。
func TestLoad_EnvOverride_Partial(t *testing.T) {
	t.Setenv("APP_ADDR", ":5555")
	// 注意：不设置 APP_DB_DRIVER，database 段应保持文件值

	dir := t.TempDir()
	path := writeFile(t, dir, "config.yaml", `
server:
  addr: ":8080"
database:
  driver: "postgres"
  dsn: "host=db"
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() 返回错误: %v", err)
	}
	// addr 被环境变量覆盖
	if cfg.Server.Addr != ":5555" {
		t.Errorf("Server.Addr 应被 APP_ADDR 覆盖，期望 \":5555\"，实际 %q", cfg.Server.Addr)
	}
	// database 未被环境变量影响，应保持文件值
	if cfg.Database.Driver != "postgres" {
		t.Errorf("未设置 APP_DB_DRIVER 时 Database.Driver 应保持文件值 \"postgres\"，实际 %q", cfg.Database.Driver)
	}
}

// TestLoad_EnvOverride_LogCallerFalse APP_LOG_CALLER=false 时应禁用 caller 输出。
// 注意源码中 APP_LOG_CALLER 的逻辑是 `if v == "false" { cfg.Log.Caller = false }`，
// 因此只有显式设为 "false" 才会关闭，其他值（包括不设置）不影响。
func TestLoad_EnvOverride_LogCallerFalse(t *testing.T) {
	t.Setenv("APP_LOG_CALLER", "false")

	dir := t.TempDir()
	// 文件中 caller=true
	path := writeFile(t, dir, "config.yaml", `
log:
  caller: true
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() 返回错误: %v", err)
	}
	if cfg.Log.Caller {
		t.Error("APP_LOG_CALLER=false 应将 Log.Caller 置为 false")
	}
}

// ============================================================
// getByPath 内部函数测试（通过 Watcher.Get 间接调用，这里直接测试）
// ============================================================

// TestGetByPath 验证 getByPath 在嵌套 map 上的路径查找逻辑。
func TestGetByPath(t *testing.T) {
	m := map[string]any{
		"a": map[string]any{
			"b": map[string]any{
				"c": "deep_value",
			},
		},
		"top": 42,
	}

	// 查找存在的深层路径
	val, ok := getByPath(m, []string{"a", "b", "c"})
	if !ok {
		t.Error("期望找到路径 a.b.c，实际未找到")
	}
	if val != "deep_value" {
		t.Errorf("a.b.c 期望 \"deep_value\"，实际 %v", val)
	}

	// 查找顶层 key
	val, ok = getByPath(m, []string{"top"})
	if !ok {
		t.Error("期望找到 top，实际未找到")
	}
	if val != 42 {
		t.Errorf("top 期望 42，实际 %v", val)
	}

	// 查找不存在的路径
	_, ok = getByPath(m, []string{"a", "x"})
	if ok {
		t.Error("期望 a.x 不存在，实际返回 ok=true")
	}

	// 中间节点不是 map 时（路径截断）
	_, ok = getByPath(m, []string{"top", "sub"})
	if ok {
		t.Error("期望 top.sub 不存在（top 是 int），实际返回 ok=true")
	}
}
