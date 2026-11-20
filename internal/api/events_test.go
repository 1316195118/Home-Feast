package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestEventBrokerFanout(t *testing.T) {
	broker := newEventBroker()
	const clients = 128
	subs := make([]chan int64, clients)
	for i := range subs {
		subs[i] = broker.subscribe()
	}
	broker.publish()
	for i, ch := range subs {
		select {
		case version := <-ch:
			if version != 1 {
				t.Fatalf("客户端 %d 收到版本 %d，期望 1", i, version)
			}
		case <-time.After(time.Second):
			t.Fatalf("客户端 %d 没收到广播", i)
		}
		broker.unsubscribe(ch)
	}
}

type deadlineRecorder struct {
	header   http.Header
	body     bytes.Buffer
	deadline time.Time
	set      chan time.Time
}

func (w *deadlineRecorder) Header() http.Header { return w.header }
func (w *deadlineRecorder) WriteHeader(int)     {}
func (w *deadlineRecorder) Write(b []byte) (int, error) {
	return w.body.Write(b)
}
func (w *deadlineRecorder) Flush() {}
func (w *deadlineRecorder) SetWriteDeadline(t time.Time) error {
	w.deadline = t
	w.set <- t
	return nil
}

func TestSSEClearsServerWriteTimeout(t *testing.T) {
	rec := &deadlineRecorder{
		header: make(http.Header),
		set:    make(chan time.Time, 1),
	}
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/api/events", nil).WithContext(ctx)
	done := make(chan struct{})
	server := &Server{events: newEventBroker()}
	go func() {
		server.streamEvents(&statusWriter{ResponseWriter: rec}, req)
		close(done)
	}()

	select {
	case deadline := <-rec.set:
		if !deadline.IsZero() {
			t.Fatalf("SSE 应清除写截止时间，实际 %v", deadline)
		}
	case <-time.After(time.Second):
		t.Fatal("SSE 没有通过 ResponseController 清除写超时")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("取消请求后 SSE 没有退出")
	}
}
