// Command backend runs the HTTP REST API and the ARI Stasis application
// controller side by side.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/zfand/homephone-dev-server/internal/api"
	"github.com/zfand/homephone-dev-server/internal/ari"
	"github.com/zfand/homephone-dev-server/internal/callctl"
	"github.com/zfand/homephone-dev-server/internal/config"
	"github.com/zfand/homephone-dev-server/internal/store"
	"github.com/zfand/homephone-dev-server/internal/wiring"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("config error", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	ariClient := ari.New(cfg.ARIBaseURL, cfg.ARIUser, cfg.ARIPassword)
	devices := &wiring.DeviceLookup{DB: db}
	app := callctl.New(ariClient, devices, db, cfg.ARIAppName, logger)

	server := api.NewServer(db, cfg.APIToken, logger)
	httpServer := &http.Server{Addr: cfg.HTTPAddr, Handler: server.Handler()}

	go func() {
		logger.Info("starting HTTP API", "addr", cfg.HTTPAddr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server error", "error", err)
		}
	}()

	go func() {
		logger.Info("starting ARI Stasis application", "app", cfg.ARIAppName)
		if err := app.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("callctl app stopped", "error", err)
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
}
