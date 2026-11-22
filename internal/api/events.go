package api

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// 多人实时同步用最轻的 SSE：服务端只广播「数据变了」的版本号，不推业务数据。
// 前端收到后自己 refresh()，避免维护复杂的增量合并与写冲突策略。
type eventBroker struct {
	mu      sync.Mutex
	subs    map[chan int64]string
	version int64
	closed  bool
}

func newEventBroker() *eventBroker {
	return &eventBroker{subs: make(map[chan int64]string)}
}

func (b *eventBroker) subscribe(clientID string) chan int64 {
	ch := make(chan int64, 1)
	b.mu.Lock()
	if b.closed {
		close(ch)
		b.mu.Unlock()
		return ch
	}
	b.subs[ch] = cleanClientID(clientID)
	b.mu.Unlock()
	return ch
}

func (b *eventBroker) unsubscribe(ch chan int64) {
	b.mu.Lock()
	delete(b.subs, ch)
	b.mu.Unlock()
}

func (b *eventBroker) publish(origin string) {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.version++
	version := b.version
	origin = cleanClientID(origin)
	for ch, clientID := range b.subs {
		// 发起写入的标签页已经拿到写接口结果，不再让 SSE 回声触发第二次刷新。
		// 没有客户端 ID 的旧页面仍保持广播行为，避免升级瞬间漏同步。
		if origin != "" && clientID == origin {
			continue
		}
		select {
		case ch <- version:
		default: // 慢客户端丢掉中间事件，下一次刷新就能拿到最新数据
		}
	}
	b.mu.Unlock()
}

func (b *eventBroker) close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.closed = true
	for ch := range b.subs {
		close(ch)
	}
	b.subs = make(map[chan int64]string)
}

// streamEvents 保持一条长连接；25 秒一次注释心跳，避免中间设备把空闲连接掐掉。
func (s *Server) streamEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		jsonErr(w, http.StatusInternalServerError, "当前服务不支持事件流")
		return
	}
	// http.Server 的普通响应写超时不适合 SSE。只对这条长连接清除写截止时间，
	// 其它接口仍保留 60 秒保护；ResponseController 会穿透 statusWriter 取到底层连接。
	_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // 反代场景下禁用响应缓冲
	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprint(w, ": connected\n\n"); err != nil {
		return
	}
	flusher.Flush()

	ch := s.events.subscribe(r.URL.Query().Get("clientId"))
	defer s.events.unsubscribe(ch)
	tick := time.NewTicker(25 * time.Second)
	defer tick.Stop()
	for {
		select {
		case version, ok := <-ch:
			if !ok {
				return
			}
			if _, err := fmt.Fprintf(w, "data: {\"v\":%d}\n\n", version); err != nil {
				return
			}
			flusher.Flush()
		case <-tick.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func cleanClientID(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 80 {
		return value[:80]
	}
	return value
}

// statusWriter 让 protect() 能判断写接口是否真的成功，只对 2xx 广播。
// 同时转发 Flush，避免把 SSE 需要的 http.Flusher 接口包丢。
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap 让 http.ResponseController 能访问底层连接（SSE 需要单独清除写超时）。
func (w *statusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
