package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/neyfua/healthd/internal/server"
)

func main() {
	port := 8080
	if value := os.Getenv("PORT"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 65535 {
			log.Fatalf("invalid PORT %q: must be between 1 and 65535", value)
		}
		port = parsed
	}

	srv := server.New(port)
	errorsCh := make(chan error, 1)
	go func() {
		log.Printf("healthd listening on :%d", port)
		errorsCh <- srv.Start()
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-errorsCh:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	case sig := <-signals:
		log.Printf("received %s, shutting down", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Fatalf("shutdown error: %v", err)
		}
		log.Println("server stopped")
	}
}
