package api

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// 多人实时同步用最轻的 SSE：服务端只广播「数据变了」的版本号，不推业务数据。
// 前端收到后自己 refresh()，避免维护复杂的增量合并与写冲突策略。
type eventBroker struct {
	mu      sync.Mutex
	subs    map[chan struct{}]bool
	version int64
}

func newEventBroker() *eventBroker {
	return &eventBroker{subs: make(map[chan struct{}]bool)}
}

func (b *eventBroker) subscribe() chan struct{} {
	ch := make(chan struct{}, 1)
	b.mu.Lock()
	b.subs[ch] = true
	b.mu.Unlock()
	return ch
}

func (b *eventBroker) unsubscribe(ch chan struct{}) {
	b.mu.Lock()
	delete(b.subs, ch)
	b.mu.Unlock()
}

func (b *eventBroker) publish() {
	b.mu.Lock()
	b.version++
	for ch := range b.subs {
		select {
		case ch <- struct{}{}:
		default: // 慢客户端丢掉中间事件，下一次刷新就能拿到最新数据
		}
	}
	b.mu.Unlock()
}

func (b *eventBroker) currentVersion() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.version
}

// streamEvents 保持一条长连接；25 秒一次注释心跳，避免中间设备把空闲连接掐掉。
func (s *Server) streamEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		jsonErr(w, http.StatusInternalServerError, "当前服务不支持事件流")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // 反代场景下禁用响应缓冲
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	ch := s.events.subscribe()
	defer s.events.unsubscribe(ch)
	tick := time.NewTicker(25 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ch:
			_, _ = fmt.Fprintf(w, "data: {\"v\":%d}\n\n", s.events.currentVersion())
			flusher.Flush()
		case <-tick.C:
			_, _ = fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
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
