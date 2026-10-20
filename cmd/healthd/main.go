package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/neyfua/healthd/internal/server"
)

const healthPath = "/health"

func portFromEnv() (int, error) {
	value := os.Getenv("PORT")
	if value == "" {
		return 8080, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 || parsed > 65535 {
		return 0, fmt.Errorf("invalid PORT %q: must be between 1 and 65535", value)
	}
	return parsed, nil
}

// checkHealth probes the local /health endpoint and reports status to stdout.
// Used as the container healthcheck so orchestrators know when the process
// is actually serving traffic.
func checkHealth(port int) error {
	return checkHealthURL("http://" + net.JoinHostPort("127.0.0.1", strconv.Itoa(port)) + healthPath)
}

func checkHealthURL(url string) error {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health endpoint returned %d", resp.StatusCode)
	}
	if _, err := io.ReadAll(resp.Body); err != nil {
		return err
	}
	return nil
}

func main() {
	healthcheck := flag.Bool("healthcheck", false, "probe the local /health endpoint and exit")
	flag.Parse()

	if *healthcheck {
		port, err := portFromEnv()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err := checkHealth(port); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	port, err := portFromEnv()
	if err != nil {
		log.Fatalf("%v", err)
	}

	slog.SetDefault(newLogger())

	srv := server.New(port)
	errorsCh := make(chan error, 1)
	go func() {
		slog.Info("healthd listening", "port", port)
		errorsCh <- srv.Start()
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-errorsCh:
		if !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "err", err)
			os.Exit(1)
		}
	case sig := <-signals:
		slog.Info("shutdown signal received", "signal", sig.String())
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			slog.Error("shutdown failed", "err", err)
			os.Exit(1)
		}
		slog.Info("server stopped")
	}
}

// newLogger builds a JSON logger leveled by the LOG_LEVEL environment
// variable (debug, info, warn, error). Container platforms expect
// line-delimited JSON on stdout.
func newLogger() *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}
