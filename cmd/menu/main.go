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
	if *dataDir == "" {
		*dataDir = "data"
	}
	if *configDir == "" {
		*configDir = "config"
	}

	web, err := fs.Sub(familymenu.Assets, familymenu.WebRoot)
	if err != nil {
		log.Fatalf("读取内嵌前端失败: %v", err)
	}
	st, err := store.Open(*dataDir, *configDir)
	if err != nil {
		log.Fatalf("打开数据/配置目录失败: %v", err)
	}
	accessCode := os.Getenv("MENU_ACCESS_CODE")

	server := &http.Server{
		Addr:              *addr + ":" + *port,
		Handler:           api.New(st, web, accessCode),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       90 * time.Second,
	}

	go func() {
		log.Printf("家里吃点啥 %s 已启动: http://%s:%s  数据: %s  配置: %s", api.Version, *addr, *port, *dataDir, *configDir)
		if accessCode != "" {
			log.Printf("已启用访问口令")
		}
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
	}
}
