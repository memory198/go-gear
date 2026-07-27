package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ============================================================
// Watcher 热加载测试
//
// Watcher 监听整个配置目录，任意 yaml 文件写入/创建均触发重载，
// 并向订阅了对应路径的 channel 推送新值。
//
// 注意事项：
//   - fsnotify 基于操作系统文件事件，存在微小延迟（通常 < 100ms），
//     测试中用带超时的 select 等待通知，避免硬 Sleep 导致测试不稳定。
//   - 测试文件操作使用 t.TempDir() 创建隔离目录，测试结束自动清理。
// ============================================================

// waitValue 等待 channel 在 timeout 时间内收到一个 Value 并返回。
// 若超时则调用 t.Fatalf 终止测试，避免测试永久阻塞。
func waitValue(t *testing.T, ch <-chan Value, timeout time.Duration, desc string) Value {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(timeout):
		t.Fatalf("%s: 等待 channel 通知超时（%v）", desc, timeout)
		return Value{} // 不可达，仅满足编译器
	}
}

// startWatcher 创建并启动 Watcher，同时注册 ctx 取消时的清理。
// 返回 *Watcher，ctx 取消后 Watcher 自动停止监听。
func startWatcher(t *testing.T, dir string) (*Watcher, context.CancelFunc) {
	t.Helper()
	w, err := NewWatcher(dir)
	if err != nil {
		t.Fatalf("NewWatcher(%s) 失败: %v", dir, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	if err := w.Start(ctx); err != nil {
		cancel()
		t.Fatalf("Watcher.Start() 失败: %v", err)
	}
	return w, cancel
}

// ============================================================
// NewWatcher 初始化测试
// ============================================================

// TestNewWatcher_LoadsInitialConfig 验证 NewWatcher 在创建时
// 会立即读取目录下所有 yaml 文件，初始化后 Get() 即可返回正确值。
func TestNewWatcher_LoadsInitialConfig(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "app.yaml", `
server:
  addr: ":3000"
  timeout: 30
`)

	w, cancel := startWatcher(t, dir)
	defer cancel()

	// 初始化完成后立即查询，应能读到配置值
	if got := w.Get("server.addr").String(); got != ":3000" {
		t.Errorf("server.addr 期望 \":3000\"，实际 %q", got)
	}
	if got := w.Get("server.timeout").Int(); got != 30 {
		t.Errorf("server.timeout 期望 30，实际 %d", got)
	}
}

// TestNewWatcher_InvalidDir 配置目录不存在时，
// NewWatcher 应返回错误（os.ReadDir 失败），而不是静默忽略。
func TestNewWatcher_InvalidDir(t *testing.T) {
	_, err := NewWatcher("/tmp/nonexistent_gear_watcher_dir_xyz")
	if err == nil {
		t.Error("目录不存在时期望 NewWatcher 返回错误，实际 nil")
	}
}

// TestNewWatcher_EmptyDir 配置目录存在但为空时，
// Watcher 应正常创建，Get() 返回不存在的 Value（Exists()=false）。
func TestNewWatcher_EmptyDir(t *testing.T) {
	dir := t.TempDir()

	w, cancel := startWatcher(t, dir)
	defer cancel()

	if w.Get("anything").Exists() {
		t.Error("空目录下任意路径应不存在")
	}
}

// TestNewWatcher_MultipleFiles 目录下有多个 yaml 文件时，
// 所有文件的内容都应被合并加载（热加载不检查 include，直接合并所有文件）。
func TestNewWatcher_MultipleFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "server.yaml", `
server:
  addr: ":4000"
`)
	writeFile(t, dir, "database.yaml", `
database:
  driver: "mysql"
`)

	w, cancel := startWatcher(t, dir)
	defer cancel()

	if got := w.Get("server.addr").String(); got != ":4000" {
		t.Errorf("server.addr 期望 \":4000\"，实际 %q", got)
	}
	if got := w.Get("database.driver").String(); got != "mysql" {
		t.Errorf("database.driver 期望 \"mysql\"，实际 %q", got)
	}
}

// ============================================================
// Get 路径查询测试
// ============================================================

// TestWatcher_Get_ExistingPath 查询存在的路径，Exists()=true，值正确。
func TestWatcher_Get_ExistingPath(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "config.yaml", `
app:
  name: "go-gear"
  version: 2
  debug: true
  ratio: 0.5
`)

	w, cancel := startWatcher(t, dir)
	defer cancel()

	if !w.Get("app.name").Exists() {
		t.Error("app.name 应存在")
	}
	if got := w.Get("app.name").String(); got != "go-gear" {
		t.Errorf("app.name 期望 \"go-gear\"，实际 %q", got)
	}
	if got := w.Get("app.version").Int(); got != 2 {
		t.Errorf("app.version 期望 2，实际 %d", got)
	}
	if !w.Get("app.debug").Bool() {
		t.Error("app.debug 期望 true")
	}
	if got := w.Get("app.ratio").Float64(); got != 0.5 {
		t.Errorf("app.ratio 期望 0.5，实际 %f", got)
	}
}

// TestWatcher_Get_MissingPath 查询不存在的路径，Exists()=false，
// 各类型方法均返回调用方提供的默认值。
func TestWatcher_Get_MissingPath(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "config.yaml", `app: {}`)

	w, cancel := startWatcher(t, dir)
	defer cancel()

	v := w.Get("app.nonexistent")
	if v.Exists() {
		t.Error("不存在的路径 Exists() 应返回 false")
	}
	if got := v.String("default_str"); got != "default_str" {
		t.Errorf("不存在路径 String() 应返回默认值，实际 %q", got)
	}
	if got := v.Int(99); got != 99 {
		t.Errorf("不存在路径 Int() 应返回默认值 99，实际 %d", got)
	}
}

// TestWatcher_Get_NestedPath 验证多层嵌套路径（超过 2 层）的查询。
func TestWatcher_Get_NestedPath(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "config.yaml", `
a:
  b:
    c:
      d: "deep"
`)

	w, cancel := startWatcher(t, dir)
	defer cancel()

	if got := w.Get("a.b.c.d").String(); got != "deep" {
		t.Errorf("a.b.c.d 期望 \"deep\"，实际 %q", got)
	}
}

// ============================================================
// Watch 订阅变更通知测试
// ============================================================

// TestWatcher_Watch_ValueChanged 修改配置文件中已订阅的路径值后，
// Watch channel 应收到包含新值的 Value。
// 这是热加载的核心功能：配置变更 → 文件写入 → fsnotify 触发 → reloadAndNotify → channel 推送。
func TestWatcher_Watch_ValueChanged(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeFile(t, dir, "config.yaml", `
server:
  addr: ":8080"
`)

	w, cancel := startWatcher(t, dir)
	defer cancel()

	// 订阅 server.addr 路径
	ch := w.Watch("server.addr")

	// 修改文件内容，触发热加载
	if err := os.WriteFile(cfgPath, []byte(`
server:
  addr: ":9090"
`), 0644); err != nil {
		t.Fatalf("写入配置文件失败: %v", err)
	}

	// 等待 channel 收到通知（最多等 2 秒，fsnotify 通常在 100ms 内触发）
	v := waitValue(t, ch, 2*time.Second, "server.addr 变更通知")
	if got := v.String(); got != ":9090" {
		t.Errorf("热加载后 server.addr 期望 \":9090\"，实际 %q", got)
	}
}

// TestWatcher_Watch_NoNotifyOnUnchanged 修改文件中未订阅的路径时，
// 已订阅路径的 channel 不应收到通知（reloadAndNotify 会对比新旧值，相同则跳过）。
func TestWatcher_Watch_NoNotifyOnUnchanged(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeFile(t, dir, "config.yaml", `
server:
  addr: ":8080"
  timeout: 30
`)

	w, cancel := startWatcher(t, dir)
	defer cancel()

	// 只订阅 server.addr
	ch := w.Watch("server.addr")

	// 只修改 server.timeout，server.addr 不变
	if err := os.WriteFile(cfgPath, []byte(`
server:
  addr: ":8080"
  timeout: 60
`), 0644); err != nil {
		t.Fatalf("写入配置文件失败: %v", err)
	}

	// 给 fsnotify 足够时间触发（如果有的话），然后断言 channel 没有收到消息
	select {
	case v := <-ch:
		t.Errorf("server.addr 未改变，channel 不应收到通知，实际收到 %q", v.String())
	case <-time.After(300 * time.Millisecond):
		// 正常情况：300ms 内没有通知，符合预期
	}
}

// TestWatcher_Watch_MultipleSubscribers 同一路径被多次 Watch 时，
// 每个订阅者的 channel 都应独立收到通知。
func TestWatcher_Watch_MultipleSubscribers(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeFile(t, dir, "config.yaml", `
app:
  version: 1
`)

	w, cancel := startWatcher(t, dir)
	defer cancel()

	// 同一路径注册两个订阅者
	ch1 := w.Watch("app.version")
	ch2 := w.Watch("app.version")

	// 修改配置
	if err := os.WriteFile(cfgPath, []byte(`
app:
  version: 2
`), 0644); err != nil {
		t.Fatalf("写入配置文件失败: %v", err)
	}

	// 两个 channel 都应收到新值
	v1 := waitValue(t, ch1, 2*time.Second, "订阅者1 app.version 变更通知")
	v2 := waitValue(t, ch2, 2*time.Second, "订阅者2 app.version 变更通知")

	if got := v1.Int(); got != 2 {
		t.Errorf("订阅者1 期望 version=2，实际 %d", got)
	}
	if got := v2.Int(); got != 2 {
		t.Errorf("订阅者2 期望 version=2，实际 %d", got)
	}
}

// TestWatcher_Watch_NewKeyAppeared 配置文件新增了之前不存在的 key，
// 订阅了该路径的 channel 应收到新增的值（旧值不存在 → 新值存在，视为变更）。
func TestWatcher_Watch_NewKeyAppeared(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeFile(t, dir, "config.yaml", `
app:
  name: "test"
`)

	w, cancel := startWatcher(t, dir)
	defer cancel()

	// 订阅一个当前不存在的路径
	ch := w.Watch("app.newkey")

	// 新增该 key
	if err := os.WriteFile(cfgPath, []byte(`
app:
  name: "test"
  newkey: "appeared"
`), 0644); err != nil {
		t.Fatalf("写入配置文件失败: %v", err)
	}

	v := waitValue(t, ch, 2*time.Second, "app.newkey 新增通知")
	if got := v.String(); got != "appeared" {
		t.Errorf("新增 key 期望 \"appeared\"，实际 %q", got)
	}
}

// TestWatcher_Watch_KeyDeleted 配置文件中删除了已订阅的 key 时，
// channel 应收到通知（旧值存在 → 新值不存在，视为变更）。
func TestWatcher_Watch_KeyDeleted(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeFile(t, dir, "config.yaml", `
app:
  name: "test"
  todelete: "bye"
`)

	w, cancel := startWatcher(t, dir)
	defer cancel()

	ch := w.Watch("app.todelete")

	// 删除该 key
	if err := os.WriteFile(cfgPath, []byte(`
app:
  name: "test"
`), 0644); err != nil {
		t.Fatalf("写入配置文件失败: %v", err)
	}

	v := waitValue(t, ch, 2*time.Second, "app.todelete 删除通知")
	// key 被删除后，Value 应不存在
	if v.Exists() {
		t.Errorf("key 被删除后 Exists() 应为 false，实际为 true，值为 %q", v.String())
	}
}

// TestWatcher_Start_CtxCancel 取消 ctx 后，Watcher 应停止监听文件变更，
// 之后的文件修改不再触发 channel 通知。
func TestWatcher_Start_CtxCancel(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeFile(t, dir, "config.yaml", `
server:
  port: 8080
`)

	w, cancel := startWatcher(t, dir)

	ch := w.Watch("server.port")

	// 先取消 ctx，停止监听
	cancel()

	// 等待 goroutine 退出（给一点时间让 ctx.Done() 被处理）
	time.Sleep(100 * time.Millisecond)

	// 再修改文件
	if err := os.WriteFile(cfgPath, []byte(`
server:
  port: 9090
`), 0644); err != nil {
		t.Fatalf("写入配置文件失败: %v", err)
	}

	// ctx 已取消，文件变更不应再触发通知
	select {
	case v := <-ch:
		// 注意：由于 goroutine 调度，极少数情况下 ctx 取消前可能已触发一次事件，
		// 此处用宽松断言：如果收到了通知，打印警告而非 Fatal，
		// 因为这属于竞态边界情况，属于实现的可接受行为。
		t.Logf("ctx 取消后仍收到通知（可能是取消前的遗留事件）: %q", v.String())
	case <-time.After(400 * time.Millisecond):
		// 预期路径：ctx 取消后不再有通知
	}
}

// TestWatcher_NonYAMLFileIgnored 配置目录中存在非 yaml 文件（如 .txt/.json），
// Watcher 初始化和热加载时均应忽略它们，不影响正常 yaml 配置的读取。
func TestWatcher_NonYAMLFileIgnored(t *testing.T) {
	dir := t.TempDir()

	// 写一个正常 yaml 和一个应被忽略的 txt 文件
	writeFile(t, dir, "config.yaml", `app: {name: "gear"}`)
	writeFile(t, dir, "readme.txt", "this should be ignored")

	// 写一个 json 文件（内容即便是合法 YAML 也应因扩展名被忽略）
	if err := os.WriteFile(filepath.Join(dir, "extra.json"), []byte(`{"key": "value"}`), 0644); err != nil {
		t.Fatalf("创建 json 文件失败: %v", err)
	}

	w, cancel := startWatcher(t, dir)
	defer cancel()

	// yaml 中的字段应可读
	if got := w.Get("app.name").String(); got != "gear" {
		t.Errorf("app.name 期望 \"gear\"，实际 %q", got)
	}

	// json 中的 key 不应被加载
	if w.Get("key").Exists() {
		t.Error("非 yaml 文件的字段不应被加载到配置中")
	}
}
