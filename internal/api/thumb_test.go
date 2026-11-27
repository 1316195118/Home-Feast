package api

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"familymenu/internal/store"
)

func testJPEG(t *testing.T, tint color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1200, 900))
	for y := 0; y < 900; y++ {
		for x := 0; x < 1200; x++ {
			img.SetRGBA(x, y, color.RGBA{
				R: uint8((int(tint.R) + x/12) % 256),
				G: uint8((int(tint.G) + y/12) % 256),
				B: tint.B,
				A: 255,
			})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 88}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// 构造一个只有 IHDR/IEND 的极小 PNG，但头部声明超大尺寸。
// image.DecodeConfig 能读出尺寸，image.Decode 才会分配像素——这正是
// 「压缩后几 MB、解码后几个 GB」的攻击面，必须在 Decode 之前拦住。
func hugeDimPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	buf.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})
	chunk := func(kind string, payload []byte) {
		body := append([]byte(kind), payload...)
		crc := crc32.ChecksumIEEE(body)
		var b [4]byte
		binary.BigEndian.PutUint32(b[:], uint32(len(payload)))
		buf.Write(b[:])
		buf.Write(body)
		binary.BigEndian.PutUint32(b[:], crc)
		buf.Write(b[:])
	}
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:4], uint32(w))
	binary.BigEndian.PutUint32(ihdr[4:8], uint32(h))
	ihdr[8] = 8  // bit depth
	ihdr[9] = 2  // truecolor
	chunk("IHDR", ihdr)
	chunk("IEND", nil)
	return buf.Bytes()
}

func TestRenderThumbRejectsHugeDimensionsBeforeDecode(t *testing.T) {
	// 20000×20000 全白图压缩后仅几十字节，但解码会分配数 GB 并 OOM 整个进程。
	huge := hugeDimPNG(t, 20000, 20000)
	if len(huge) > 4096 {
		t.Fatalf("测试图应极小，实际 %d 字节", len(huge))
	}
	if _, err := renderThumbJPEG(huge); err == nil {
		t.Fatal("超大尺寸图片必须在解码前被拒绝")
	}
	// 正常尺寸仍可用
	normal := testJPEG(t, color.RGBA{R: 200, G: 100, B: 50, A: 255})
	if _, err := renderThumbJPEG(normal); err != nil {
		t.Fatalf("正常图片应能生成缩略图: %v", err)
	}
}

func TestServeUploadGeneratesMissingThumbInMemory(t *testing.T) {
	root := t.TempDir()
	st, err := store.Open(filepath.Join(root, "data"), filepath.Join(root, "config"))
	if err != nil {
		t.Fatal(err)
	}
	uploadDir := filepath.Join(st.Dir(), "uploads")
	source := filepath.Join(uploadDir, "imglegacy.jpg")
	original := testJPEG(t, color.RGBA{R: 190, G: 90, B: 50, A: 255})
	if err := os.WriteFile(source, original, 0o644); err != nil {
		t.Fatal(err)
	}

	s := &Server{
		st: st, thumbCache: make(map[string]*generatedThumb),
		thumbLoads: make(map[string]*generatedThumbLoad),
	}

	req := httptest.NewRequest(http.MethodGet, "/uploads/thumb/imglegacy.jpg", nil)
	rec := httptest.NewRecorder()
	s.serveUpload(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("缺失缩略图应即时生成，状态码=%d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "image/jpeg" {
		t.Fatalf("Content-Type=%q", got)
	}
	if rec.Body.Len() == 0 || rec.Body.Len() >= len(original) {
		t.Fatalf("缩略图体积异常：thumb=%d original=%d", rec.Body.Len(), len(original))
	}
	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatal("动态缩略图缺少 ETag")
	}
	if _, err := os.Stat(filepath.Join(uploadDir, "thumb", "imglegacy.jpg")); !os.IsNotExist(err) {
		t.Fatalf("动态缩略图不应写入 /data 卷，stat err=%v", err)
	}

	req = httptest.NewRequest(http.MethodGet, "/uploads/thumb/imglegacy.jpg", nil)
	req.Header.Set("If-None-Match", etag)
	rec = httptest.NewRecorder()
	s.serveUpload(rec, req)
	if rec.Code != http.StatusNotModified {
		t.Fatalf("相同缩略图应返回 304，实际 %d", rec.Code)
	}
}

func TestGeneratedThumbSingleFlightConcurrent(t *testing.T) {
	root := t.TempDir()
	st, err := store.Open(filepath.Join(root, "data"), filepath.Join(root, "config"))
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(st.Dir(), "uploads", "imgconcurrent.jpg")
	if err := os.WriteFile(source, testJPEG(t, color.RGBA{R: 80, G: 170, B: 85, A: 255}), 0o644); err != nil {
		t.Fatal(err)
	}
	s := &Server{
		st: st, thumbCache: make(map[string]*generatedThumb),
		thumbLoads: make(map[string]*generatedThumbLoad),
	}

	var wg sync.WaitGroup
	errs := make(chan string, 32)
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/uploads/thumb/imgconcurrent.jpg", nil)
			rec := httptest.NewRecorder()
			s.serveUpload(rec, req)
			if rec.Code != http.StatusOK || rec.Body.Len() == 0 {
				errs <- "并发请求失败"
			}
		}()
	}
	wg.Wait()
	close(errs)
	for msg := range errs {
		t.Error(msg)
	}
}

func TestGeneratedThumbRebuildsWhenSourceChanges(t *testing.T) {
	root := t.TempDir()
	st, err := store.Open(filepath.Join(root, "data"), filepath.Join(root, "config"))
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(st.Dir(), "uploads", "imgedit.jpg")
	if err := os.WriteFile(source, testJPEG(t, color.RGBA{R: 210, G: 90, B: 60, A: 255}), 0o644); err != nil {
		t.Fatal(err)
	}
	s := &Server{
		st: st, thumbCache: make(map[string]*generatedThumb),
		thumbLoads: make(map[string]*generatedThumbLoad),
	}
	first, err := s.generatedUploadThumb("imgedit")
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(source, testJPEG(t, color.RGBA{R: 70, G: 120, B: 220, A: 255}), 0o644); err != nil {
		t.Fatal(err)
	}
	nextTime := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(source, nextTime, nextTime); err != nil {
		t.Fatal(err)
	}
	second, err := s.generatedUploadThumb("imgedit")
	if err != nil {
		t.Fatal(err)
	}
	if first.etag == second.etag {
		t.Fatal("原图变化后仍复用旧缩略图")
	}
}
