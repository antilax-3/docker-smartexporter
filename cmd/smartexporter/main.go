// Command smartexporter periodically scrapes the SMART attributes of the host's disks with smartctl and serves them
// for Prometheus.
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

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/antilax-3/docker-smartexporter/internal/config"
	"github.com/antilax-3/docker-smartexporter/internal/exporter"
	"github.com/antilax-3/docker-smartexporter/internal/smartctl"
)

const configPath = "/config/smartexporter.json"

func main() {
	log.SetFlags(0)

	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run() error {
	cfg, warning, err := config.Load(configPath)
	if err != nil {
		log.Print(err)
	}

	if warning != "" {
		log.Print(warning)
	}

	e, err := exporter.New(&smartctl.Client{Run: smartctl.Exec}, cfg.Attributes, log.Default())
	if err != nil {
		return err
	}

	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		e,
	)

	mux := http.NewServeMux()
	mux.Handle("GET /{$}", exporter.Index())
	mux.Handle("GET /metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{ErrorLog: log.Default()}))

	server := &http.Server{
		Addr:              ":" + strconv.Itoa(cfg.ListenPort()),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      time.Minute,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errs := make(chan error, 1)

	go func() {
		log.Printf("Running smartexporter. Listening on port %d.", cfg.ListenPort())
		errs <- server.ListenAndServe()
	}()

	go e.Run(ctx, cfg.Interval())

	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
	}

	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdown); err != nil {
		return err
	}

	if err := <-errs; !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}
