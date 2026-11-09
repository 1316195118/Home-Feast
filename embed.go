// Package familymenu 提供前端静态资源的嵌入入口。
package familymenu

import "embed"

// Assets 保存由 Vite 构建出的整棵前端目录，随二进制一起分发。
//
//go:embed all:webapp/dist
var Assets embed.FS

// WebRoot 是这些文件在嵌入文件系统里的统一前缀。
const WebRoot = "webapp/dist"
