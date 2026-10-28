package logger

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/memory198/go-gear/logger/core"
)

// 时间格式化缓存：同一秒内日期时间与时区固定，仅微秒变化
// 避免每条日志都执行 time.Format（layout 解析 + 时区计算 + string 分配）
var (
	timeCacheSec    atomic.Int64
	timeCacheOffset atomic.Int32  // 时区偏移（秒），参与缓存键；同一秒不同时区不能复用
	timeCacheHead   atomic.Pointer[[]byte] // "2006-01-02T15:04:05"
	timeCacheZone   atomic.Pointer[[]byte] // "+08:00" 或 "Z"
)

// appendTime 追加 RFC3339 微秒格式时间（例：2026-09-21T10:30:00.123456+08:00）
// 秒级日期时间与时区缓存，同一秒内零 Format 开销、零分配
func appendTime(b []byte, t time.Time) []byte {
	sec := t.Unix()
	_, offset := t.Zone()
	head := timeCacheHead.Load()
	if head == nil || timeCacheSec.Load() != sec || int(timeCacheOffset.Load()) != offset {
		h := []byte(t.Format("2006-01-02T15:04:05"))
		z := []byte(t.Format("Z07:00"))
		timeCacheHead.Store(&h)
		timeCacheZone.Store(&z)
		timeCacheOffset.Store(int32(offset))
		timeCacheSec.Store(sec)
		head = timeCacheHead.Load()
	}
	b = append(b, (*head)...)
	b = append(b, '.')
	b = appendMicro(b, t.Nanosecond()/1000)
	return append(b, (*timeCacheZone.Load())...)
}

// appendMicro 追加 6 位微秒（前导零补齐）
func appendMicro(b []byte, v int) []byte {
	var buf [6]byte
	for i := 5; i >= 0; i-- {
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return append(b, buf[:]...)
}

// bufPool 复用日志输出缓冲，避免每行日志重复分配
// 注：池中缓冲保留历史最大容量（超大日志会使缓冲长期驻留）
var bufPool = sync.Pool{
	New: func() any {
		b := make([]byte, 0, 512)
		return &b
	},
}

// encoder 日志编码器：将 Record 序列化后追加到 dst 并返回新切片
type encoder interface {
	appendTo(dst []byte, r *core.Record) []byte
}
