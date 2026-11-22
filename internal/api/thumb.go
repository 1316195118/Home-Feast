package api

import (
	"bytes"
	_ "golang.org/x/image/webp" // 注册 WebP 解码（前端上传的就是 webp）
	"image"
	"image/draw"
	_ "image/gif" // 注册 GIF 解码
	"image/jpeg"
	_ "image/png" // 注册 PNG 解码
	"os"
	"path/filepath"
)

// 列表用的缩略图宽度。卡片实际显示 200~400px，2 倍屏也够用。
const thumbWidth = 400

// thumbJPEGQuality 取 78：400px 下肉眼几乎看不出差别，单张约 20~30KB。
const thumbJPEGQuality = 78

// writeThumb 为刚上传的图片生成缩略图，写入 uploads/thumb/<base>.jpg。
//
// 只用标准库：JPEG 有编码器（WebP 只有解码器），所以缩略图统一转 JPEG。
// 缩放是自己写的双线性插值 —— 标准库的 image/draw 只有最近邻，400px 下锯齿明显，
// 而 golang.org/x/image 属于额外依赖，本项目坚持运行期零第三方包。
// 失败不影响上传本身：列表会回落到原图，只是费点流量。
func writeThumb(data []byte, uploadDir, base string) error {
	thumb, err := renderThumbJPEG(data)
	if err != nil {
		return err
	}
	dir := filepath.Join(uploadDir, "thumb")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, base+".jpg"), thumb, 0o644)
}

// renderThumbJPEG 生成列表用的 400px JPEG。上传时写入磁盘；
// 老数据缺缩略图时也复用它，在请求阶段即时生成并放入内存缓存。
func renderThumbJPEG(data []byte) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	sw, sh := b.Dx(), b.Dy()
	if sw <= 0 || sh <= 0 {
		return nil, nil
	}

	w, h := sw, sh
	if sw > thumbWidth {
		w = thumbWidth
		h = sh * thumbWidth / sw
	}
	if h < 1 {
		h = 1
	}

	// 统一转成 RGBA 再取样，避免逐像素走接口断言
	srcRGBA := image.NewRGBA(image.Rect(0, 0, sw, sh))
	draw.Draw(srcRGBA, srcRGBA.Bounds(), img, b.Min, draw.Src)

	var dst *image.RGBA
	if w == sw && h == sh {
		dst = srcRGBA
	} else {
		dst = scaleBilinear(srcRGBA, w, h)
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: thumbJPEGQuality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// scaleBilinear 用双线性插值缩放，直接操作 Pix 字节，比逐点 At/RGBAAt 快一个数量级。
func scaleBilinear(src *image.RGBA, w, h int) *image.RGBA {
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	xRatio := float64(sw) / float64(w)
	yRatio := float64(sh) / float64(h)

	for y := 0; y < h; y++ {
		sy := (float64(y)+0.5)*yRatio - 0.5
		y0 := int(sy)
		if y0 < 0 {
			y0 = 0
		}
		y1 := y0 + 1
		if y1 >= sh {
			y1 = sh - 1
		}
		fy := sy - float64(y0)
		if fy < 0 {
			fy = 0
		}

		row0 := y0 * src.Stride
		row1 := y1 * src.Stride
		outRow := y * dst.Stride

		for x := 0; x < w; x++ {
			sx := (float64(x)+0.5)*xRatio - 0.5
			x0 := int(sx)
			if x0 < 0 {
				x0 = 0
			}
			x1 := x0 + 1
			if x1 >= sw {
				x1 = sw - 1
			}
			fx := sx - float64(x0)
			if fx < 0 {
				fx = 0
			}

			i00 := row0 + x0*4
			i01 := row0 + x1*4
			i10 := row1 + x0*4
			i11 := row1 + x1*4
			p := src.Pix
			o := outRow + x*4
			for c := 0; c < 3; c++ {
				top := float64(p[i00+c])*(1-fx) + float64(p[i01+c])*fx
				bot := float64(p[i10+c])*(1-fx) + float64(p[i11+c])*fx
				dst.Pix[o+c] = uint8(top*(1-fy) + bot*fy + 0.5)
			}
			dst.Pix[o+3] = 255
		}
	}
	return dst
}
