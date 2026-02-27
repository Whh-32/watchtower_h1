package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"watchtower/internal/config"
	"watchtower/internal/database"
	"watchtower/internal/discovery"
	"watchtower/internal/hackerone"
	"watchtower/internal/healthcheck"
	"watchtower/internal/scheduler"
	"watchtower/internal/server"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Validate HackerOne token
	if cfg.HackerOneToken == "" {
		log.Fatalf("HACKERONE_TOKEN is required. Set it via environment variable or .hackerone_token file")
	}

	// Initialize database
	db, err := database.Init(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize services
	hackeroneClient := hackerone.NewClient(cfg.HackerOneToken)
	discoveryService := discovery.NewService()
	healthCheckService := healthcheck.NewService(cfg.HealthCheckTimeout, cfg.HealthCheckWorkers)

	// Initialize scheduler
	scanScheduler := scheduler.NewScheduler(db, hackeroneClient, discoveryService, healthCheckService, cfg)

	// Start web server FIRST so users can see live results
	webServer := server.NewServer(db, cfg.WebPort)
	go func() {
		log.Printf("Starting web server on port %s...", cfg.WebPort)
		log.Printf("🌐 Web interface available at: http://localhost:%s", cfg.WebPort)
		if err := webServer.Start(); err != nil {
			log.Fatalf("Failed to start web server: %v", err)
		}
	}()

	// Give web server a moment to start
	time.Sleep(1 * time.Second)

	// Run initial scan in background so web server is immediately available
	go func() {
		log.Println("🔍 Starting initial scan in background...")
		if err := scanScheduler.RunScan(); err != nil {
			log.Printf("Initial scan error: %v", err)
		} else {
			log.Println("✅ Initial scan completed!")
		}
	}()

	// Schedule daily scans
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			log.Println("Running scheduled daily scan...")
			if err := scanScheduler.RunScan(); err != nil {
				log.Printf("Scheduled scan error: %v", err)
			}
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
}
