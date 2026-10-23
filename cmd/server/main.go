package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"westeros-web/internal/assets"
	"westeros-web/internal/server"
	"westeros-web/internal/store"
)

func env(key, fallback string) string {
	if s := os.Getenv(key); s != "" {
		return s
	}
	return fallback
}
func main() {
	addr := flag.String("listen", env("LISTEN_ADDR", "127.0.0.1:18081"), "HTTP listen address")
	data := flag.String("data", env("DATA_DIR", ".local-data"), "SQLite directory")
	flag.Parse()
	db, err := store.Open(filepath.Join(*data, "westeros.sqlite"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	app := server.New(db, assets.Web())
	app.Secure = os.Getenv("COOKIE_SECURE") == "true"
	app.LocalAssets(os.Getenv("LOCAL_ASSETS_DIR"))
	httpServer := &http.Server{Addr: *addr, Handler: app, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go app.RunTimers(ctx)
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdown)
	}()
	log.Printf("Westeros Web (Go), tactical prototype: http://%s", *addr)
	if err = httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
