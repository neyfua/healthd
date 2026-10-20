package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
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
