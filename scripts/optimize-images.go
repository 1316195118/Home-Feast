//go:build ignore

// optimize-images 把 docs/artwork 下的原始 PNG 插画转成体积更小的 JPEG，
// 输出到 web/public/dishes 供前端使用。只用 Go 标准库，不引入第三方依赖。
//
// 用法（项目根目录）：go run scripts/optimize-images.go
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

const (
	srcDir  = "docs/artwork"
	dstDir  = "web/public/dishes"
	quality = 82
)

func main() {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		fail("读取素材目录", err)
	}
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		fail("创建输出目录", err)
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".png") {
			continue
		}
		in := filepath.Join(srcDir, entry.Name())
		out := filepath.Join(dstDir, strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))+".jpg")
		if err := convert(in, out); err != nil {
			fail(in, err)
		}
		before, _ := os.Stat(in)
		after, _ := os.Stat(out)
		fmt.Printf("%-18s %6.1f KB -> %6.1f KB\n", entry.Name(),
			float64(before.Size())/1024, float64(after.Size())/1024)
		count++
	}
	fmt.Printf("完成，共转换 %d 张插画。\n", count)
}

func convert(in, out string) error {
	f, err := os.Open(in)
	if err != nil {
		return err
	}
	defer f.Close()

	src, err := png.Decode(f)
	if err != nil {
		return err
	}
	// 统一铺到米白底上，避免带 alpha 的图在转 JPEG 时出现黑边。
	canvas := image.NewRGBA(src.Bounds())
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: color.RGBA{R: 251, G: 246, B: 238, A: 255}}, image.Point{}, draw.Src)
	draw.Draw(canvas, canvas.Bounds(), src, src.Bounds().Min, draw.Over)

	w, err := os.Create(out)
	if err != nil {
		return err
	}
	defer w.Close()
	return jpeg.Encode(w, canvas, &jpeg.Options{Quality: quality})
}

func fail(what string, err error) {
	fmt.Fprintf(os.Stderr, "处理 %s 失败：%v\n", what, err)
	os.Exit(1)
}
