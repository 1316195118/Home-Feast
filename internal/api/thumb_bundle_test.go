package api

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"familymenu/internal/store"
)

func TestThumbBundleReadsUploadsAndEmbeddedAssets(t *testing.T) {
	root := t.TempDir()
	st, err := store.Open(filepath.Join(root, "data"), filepath.Join(root, "config"))
	if err != nil {
		t.Fatal(err)
	}
	thumbDir := filepath.Join(st.Dir(), "uploads", "thumb")
	if err := os.MkdirAll(thumbDir, 0o755); err != nil {
		t.Fatal(err)
	}
	uploadThumb := []byte("jpeg-thumb")
	if err := os.WriteFile(filepath.Join(thumbDir, "imgbundle.jpg"), uploadThumb, 0o644); err != nil {
		t.Fatal(err)
	}
	s := &Server{
		st: st,
		web: fstest.MapFS{
			"dish-photo/thumb/demo.webp": {Data: []byte("webp-thumb")},
		},
		thumbCache: make(map[string]*generatedThumb),
		thumbLoads: make(map[string]*generatedThumbLoad),
	}

	body, _ := json.Marshal(thumbBundleRequest{URLs: []string{
		"/uploads/thumb/imgbundle.jpg",
		"/dish-photo/thumb/demo.webp",
		"/uploads/../secret.jpg",
		"/uploads/thumb/imgbundle.jpg",
	}})
	req := httptest.NewRequest(http.MethodPost, "/thumb-bundle", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.serveThumbBundle(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("状态码=%d body=%s", rec.Code, rec.Body.String())
	}
	data := rec.Body.Bytes()
	if len(data) < 4 {
		t.Fatal("缩略图包过短")
	}
	headerLen := int(binary.BigEndian.Uint32(data[:4]))
	if headerLen <= 0 || 4+headerLen > len(data) {
		t.Fatalf("索引长度异常=%d total=%d", headerLen, len(data))
	}
	var entries []thumbBundleEntry
	if err := json.Unmarshal(data[4:4+headerLen], &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("应去重并忽略穿越路径，entries=%+v", entries)
	}
	payload := data[4+headerLen:]
	if !bytes.Equal(payload[entries[0].Offset:entries[0].Offset+entries[0].Size], uploadThumb) {
		t.Fatal("上传缩略图内容不正确")
	}
	if got := string(payload[entries[1].Offset : entries[1].Offset+entries[1].Size]); got != "webp-thumb" {
		t.Fatalf("内嵌缩略图内容=%q", got)
	}
}

func TestThumbBundleRejectsOversizedAndCrossOriginRequests(t *testing.T) {
	s := &Server{}
	tooMany := make([]string, maxThumbBundleItems+1)
	body, _ := json.Marshal(thumbBundleRequest{URLs: tooMany})
	req := httptest.NewRequest(http.MethodPost, "/thumb-bundle", strings.NewReader(string(body)))
	rec := httptest.NewRecorder()
	s.serveThumbBundle(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("超量请求应拒绝，实际 %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/thumb-bundle", strings.NewReader(`{"urls":["/dishes/a.webp"]}`))
	req.Header.Set("Origin", "https://evil.example")
	rec = httptest.NewRecorder()
	s.serveThumbBundle(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("跨站请求应拒绝，实际 %d", rec.Code)
	}
}
