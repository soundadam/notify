package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/soundadam/notify/internal/server"
)

func main() {
	cfg, err := server.ConfigFromEnv()
	if err != nil {
		log.Fatalf("配置无效: %v", err)
	}

	httpClient := server.ProxyHTTPClient(15 * time.Second)
	srv, err := server.New(cfg, httpClient)
	if err != nil {
		log.Fatalf("初始化失败: %v", err)
	}

	httpSrv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("notify 监听 %s", cfg.ListenAddr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP 服务退出: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("关闭失败: %v", err)
	}
}
