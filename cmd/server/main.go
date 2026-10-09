package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"pdf-golang-zip/internal/config"
	"pdf-golang-zip/internal/httpapi"
	"pdf-golang-zip/internal/observability"
	"pdf-golang-zip/internal/storage"
)

func main() {
	syscall.Umask(0077)
	logger := observability.Logger()
	c, err := config.Load()
	if err != nil {
		logger.Error("invalid_config")
		os.Exit(1)
	}
	store := storage.Store{Root: c.TempDir}
	if err := store.Startup(); err != nil {
		logger.Error("storage_unavailable")
		os.Exit(1)
	}
	api := httpapi.New(c, store, httpapi.ProcessWorker{Config: c}, logger)
	srv := &http.Server{Addr: c.ListenAddr, Handler: api.Handler(), ReadHeaderTimeout: c.ReadHeaderTimeout, ReadTimeout: c.ReadTimeout, WriteTimeout: c.WriteTimeout, MaxHeaderBytes: 1 << 20}
	signals, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	done := make(chan error, 1)
	go func() { done <- srv.ListenAndServe() }()
	logger.Info("server_started", "listen", c.ListenAddr)
	select {
	case err := <-done:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server_failed")
			os.Exit(1)
		}
	case <-signals.Done():
		api.Stop()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			_ = srv.Close()
		}
		<-done
	}
	logger.Info("server_stopped")
}
