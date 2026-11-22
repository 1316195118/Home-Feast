// Command menu 是家庭点菜系统的单文件服务端：内嵌前端、只写本地 JSON。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"familymenu"
	"familymenu/internal/api"
	"familymenu/internal/store"
)

func main() {
	addr := flag.String("addr", "0.0.0.0", "监听地址，只给本机用可设 127.0.0.1")
	port := flag.String("port", "8090", "监听端口")
	dataDir := flag.String("data", "", "用户数据目录，默认程序旁的 data/")
	configDir := flag.String("config", "", "配置目录（家庭设置等），默认程序旁的 config/，与 data 平级")
	tlsCert := flag.String("tls-cert", "", "可选：HTTPS 证书路径，启用后可协商 HTTP/2")
	tlsKey := flag.String("tls-key", "", "可选：HTTPS 私钥路径，必须与 --tls-cert 一起使用")
	showVersion := flag.Bool("version", false, "打印版本号")
	flag.Parse()

	if *showVersion {
		fmt.Println("family-menu", api.Version)
		return
	}

	explicit := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) { explicit[f.Name] = true })
	if v := os.Getenv("MENU_ADDR"); v != "" && !explicit["addr"] {
		*addr = v
	}
	if v := os.Getenv("MENU_PORT"); v != "" && !explicit["port"] {
		*port = v
	}
	if v := os.Getenv("MENU_DATA_DIR"); v != "" && !explicit["data"] {
		*dataDir = v
	}
	if v := os.Getenv("MENU_CONFIG_DIR"); v != "" && !explicit["config"] {
		*configDir = v
	}
	if v := os.Getenv("MENU_TLS_CERT"); v != "" && !explicit["tls-cert"] {
		*tlsCert = v
	}
	if v := os.Getenv("MENU_TLS_KEY"); v != "" && !explicit["tls-key"] {
		*tlsKey = v
	}
	if *dataDir == "" {
		*dataDir = "data"
	}
	if *configDir == "" {
		*configDir = "config"
	}
	if (*tlsCert == "") != (*tlsKey == "") {
		log.Fatal("启用 HTTPS 时必须同时提供 --tls-cert 与 --tls-key")
	}

	web, err := fs.Sub(familymenu.Assets, familymenu.WebRoot)
	if err != nil {
		log.Fatalf("读取内嵌前端失败: %v", err)
	}
	st, err := store.Open(*dataDir, *configDir)
	if err != nil {
		log.Fatalf("打开数据/配置目录失败: %v", err)
	}
	// 在线更新后由新进程自检：若版本对不上（说明更新失败被重新拉起），自动回滚
	api.CheckRecovery(*dataDir, "")
	accessCode := os.Getenv("MENU_ACCESS_CODE")

	handler := api.New(st, web, accessCode)
	server := &http.Server{
		Addr:              *addr + ":" + *port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       2 * time.Minute, // 给手机慢速上传照片留足时间
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       2 * time.Minute, // 多端浏览时复用 keep-alive 连接
		MaxHeaderBytes:    64 << 10,
	}
	// SSE 是长连接。退出时先结束事件流，否则 Shutdown 会一直等在线设备断开。
	if closer, ok := handler.(interface{ Close() }); ok {
		server.RegisterOnShutdown(closer.Close)
	}

	go func() {
		scheme := "http"
		if *tlsCert != "" {
			// net/http 会在证书有效且客户端支持时自动协商 HTTP/2；
			// 同一连接可多路复用图片请求，不再受 HTTP/1.1 单域名 6 连接限制。
			scheme = "https"
		}
		log.Printf("家里吃点啥 %s 已启动: %s://%s:%s  数据: %s  配置: %s", api.Version, scheme, *addr, *port, *dataDir, *configDir)
		if accessCode != "" {
			log.Printf("已启用访问口令")
		}
		var err error
		if scheme == "https" {
			err = server.ListenAndServeTLS(*tlsCert, *tlsKey)
		} else {
			err = server.ListenAndServe()
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("服务启动失败: %v", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	log.Printf("收到退出信号，正在收尾…")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("关闭超时: %v", err)
		_ = server.Close()
	}
}
