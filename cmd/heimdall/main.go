package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/blacken57/heimdall/internal/api"
	"github.com/blacken57/heimdall/internal/checker"
	"github.com/blacken57/heimdall/internal/config"
	"github.com/blacken57/heimdall/internal/db"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer database.Close()

	// Sync services table with config.
	serviceIDs := make(map[string]int64, len(cfg.Services))
	names := make([]string, 0, len(cfg.Services))
	for _, svc := range cfg.Services {
		id, err := database.UpsertService(svc.Name, svc.URL)
		if err != nil {
			log.Fatalf("upsert service %q: %v", svc.Name, err)
		}
		serviceIDs[svc.Name] = id
		names = append(names, svc.Name)
	}
	if err := database.DeleteServicesNotIn(names); err != nil {
		log.Fatalf("prune services: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	chk := checker.New(cfg, database, serviceIDs)
	go chk.Run(ctx)

	// Periodic purge of old check records.
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := database.PurgeOldChecks(cfg.DataRetentionDays); err != nil {
					log.Printf("purge old checks: %v", err)
				}
			}
		}
	}()

	srv := api.New(cfg, database)
	httpServer := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: srv,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("heimdall listening on :%s", cfg.Port)
		if cfg.BasicAuthEnabled() {
			log.Printf("basic auth enabled for user %q", cfg.HeimdallUser)
		}
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server: %v", err)
		}
	}()

	<-quit
	log.Println("shutting down…")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
