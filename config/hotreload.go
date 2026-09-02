package config

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

// Watcher 热加载管理器
// 监听整个配置目录，任意 yaml 文件变化都会触发重载
type Watcher struct {
	configDir string
	mu        sync.RWMutex
	raw       map[string]any        // 当前合并后的原始 map
	subs      map[string][]chan any // path -> 订阅者 channel 列表
}

// NewWatcher 创建热加载管理器，初始加载配置目录下所有 yaml 文件
func NewWatcher(configDir string) (*Watcher, error) {
	w := &Watcher{
		configDir: configDir,
		raw:       make(map[string]any),
		subs:      make(map[string][]chan any),
	}
	if err := w.reload(); err != nil {
		return nil, err
	}
	return w, nil
}

// Get 获取指定路径的值，支持链式调用
// 用法：watcher.Get("params.timeout.read").Int(30)
func (w *Watcher) Get(path string) Value {
	w.mu.RLock()
	defer w.mu.RUnlock()
	raw, ok := getByPath(w.raw, strings.Split(path, "."))
	return newValue(raw, ok)
}

// watchEvent 内部事件结构，同时携带原始值和路径是否存在的标志，
// 避免 key 被删除时 ok 信息丢失。
type watchEvent struct {
	raw any
	ok  bool
}

// Watch 订阅指定路径，值变化时推送新 Value 到 channel。
// 当路径对应的 key 被删除时，推送 Exists()=false 的 Value。
func (w *Watcher) Watch(path string) <-chan Value {
	ch := make(chan Value, 1)
	w.mu.Lock()
	// 内部使用 watchEvent channel，携带完整的 raw+ok 信息
	inner := make(chan any, 1)
	w.subs[path] = append(w.subs[path], inner)
	w.mu.Unlock()

	go func() {
		for e := range inner {
			if evt, ok := e.(watchEvent); ok {
				ch <- newValue(evt.raw, evt.ok)
			}
		}
	}()
	return ch
}

// Start 启动文件监听，ctx 取消时停止
func (w *Watcher) Start(ctx context.Context) error {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create fsnotify watcher: %w", err)
	}
	if err := fsw.Add(w.configDir); err != nil {
		fsw.Close()
		return fmt.Errorf("watch dir %s: %w", w.configDir, err)
	}

	go func() {
		defer fsw.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-fsw.Events:
				if !ok {
					return
				}
				// 只处理 yaml 文件的写入/创建事件
				if !isYAML(event.Name) {
					continue
				}
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
					log.Printf("config: hot reload triggered by %s", filepath.Base(event.Name))
					w.reloadAndNotify()
				}
			case err, ok := <-fsw.Errors:
				if !ok {
					return
				}
				log.Printf("config: watcher error: %v", err)
			}
		}
	}()
	return nil
}

// reload 重新加载配置目录下所有 yaml 文件
func (w *Watcher) reload() error {
	entries, err := os.ReadDir(w.configDir)
	if err != nil {
		return fmt.Errorf("read config dir: %w", err)
	}

	merged := make(map[string]any)
	for _, entry := range entries {
		if entry.IsDir() || !isYAML(entry.Name()) {
			continue
		}
		path := filepath.Join(w.configDir, entry.Name())
		data, err := readYAML(path)
		if err != nil {
			return err
		}
		// 热加载不检查 include，也不 panic（静态配置已做检查）
		for k, v := range data {
			if k == "include" {
				continue
			}
			merged[k] = v
		}
	}

	w.mu.Lock()
	w.raw = merged
	w.mu.Unlock()
	return nil
}

// reloadAndNotify 重新加载并通知变化的订阅路径
func (w *Watcher) reloadAndNotify() {
	w.mu.RLock()
	oldRaw := copyMap(w.raw)
	w.mu.RUnlock()

	if err := w.reload(); err != nil {
		log.Printf("config: reload error: %v", err)
		return
	}

	w.mu.RLock()
	defer w.mu.RUnlock()

	for path, channels := range w.subs {
		keys := strings.Split(path, ".")
		newVal, newOk := getByPath(w.raw, keys)
		oldVal, oldOk := getByPath(oldRaw, keys)

		if newOk != oldOk || fmt.Sprint(newVal) != fmt.Sprint(oldVal) {
			evt := watchEvent{raw: newVal, ok: newOk}
			for _, ch := range channels {
				select {
				case ch <- evt:
				default:
					// channel 已满时先排空旧值再写入，保证订阅者总能拿到最新值
					select {
					case <-ch:
					default:
					}
					ch <- evt
				}
			}
		}
	}
}

// ---- 工具函数 ----

func getByPath(m map[string]any, keys []string) (any, bool) {
	var cur any = m
	for _, k := range keys {
		mp, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = mp[k]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

func isYAML(name string) bool {
	ext := filepath.Ext(name)
	return ext == ".yaml" || ext == ".yml"
}

func readYAML(path string) (map[string]any, error) {
	data, err := readFile(path)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return m, nil
}

func copyMap(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
