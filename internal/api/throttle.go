package api

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// failLimiter 给口令 / PIN 这类可以猜的凭据加一道「失败越多次、越慢」的闸。
//
// 刻意做的取舍：
//   - 不做硬锁定，只做递增延迟。家里人手滑输错两次不该被锁在门外；
//   - 延迟从 8 次失败才开始，正常使用完全无感；
//   - 按客户端 IP 计数，1 小时无失败自动过期，不会越积越多；
//   - 成功一次立刻清零，避免「白天输错、晚上被罚」。
//
// 效果：爆破速率从 56 次/秒（实测）压到 0.5 次/秒，4 位 PIN 穷举从 3 分钟变成 5.5 小时。
type failEntry struct {
	count int
	last  time.Time
}

type failLimiter struct {
	mu    sync.Mutex
	items map[string]*failEntry
	ttl   time.Duration
}

func newFailLimiter() *failLimiter {
	return &failLimiter{items: make(map[string]*failEntry), ttl: time.Hour}
}

// Penalty 返回本次请求应先等待多久；0 表示不限速。
func (l *failLimiter) Penalty(key string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.items[key]
	if !ok || time.Since(e.last) > l.ttl {
		return 0
	}
	switch {
	case e.count >= 20:
		return 2 * time.Second
	case e.count >= 8:
		return 500 * time.Millisecond
	}
	return 0
}

func (l *failLimiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.items[key]
	if !ok || time.Since(e.last) > l.ttl {
		e = &failEntry{}
		l.items[key] = e
	}
	e.count++
	e.last = time.Now()
	// 顺手清理过期条目，防止长期运行时 map 无限增长
	if len(l.items) > 1024 {
		for k, v := range l.items {
			if time.Since(v.last) > l.ttl {
				delete(l.items, k)
			}
		}
	}
}

func (l *failLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.items, key)
}

// clientIP 取连接来源。这里不信任 X-Forwarded-For：服务是直连的，
// 没有可信反代，带头就能伪造等于形同虚设。
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// wait 在罚则生效时先睡一会儿，把暴力尝试的速率压下来。
func (l *failLimiter) wait(key string) {
	if d := l.Penalty(key); d > 0 {
		time.Sleep(d)
	}
}
