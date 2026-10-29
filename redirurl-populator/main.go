// Command redirurl-populator keeps an OpenShift OAuthClient's redirectURIs in
// sync with a required set of URLs on a remote (central) cluster.
//
// It is intended to run alongside each spoke cluster and, on an interval,
// ensure the spoke's OAuth redirect URLs are registered on the central OAuth
// identity provider so the spoke can authenticate against it.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/suvarineko/redirurl-populator/internal/config"
	"github.com/suvarineko/redirurl-populator/internal/kube"
	"github.com/suvarineko/redirurl-populator/internal/reconciler"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	os.Exit(run())
}

func run() int {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "-v", "--version", "version":
			fmt.Println("redirurl-populator", version)
			return 0
		}
	}

	// Bootstrap logger so configuration errors are logged consistently; it is
	// replaced once the validated configuration is available.
	logger := newLogger(strings.ToLower(os.Getenv("LOG_LEVEL")), strings.ToLower(os.Getenv("LOG_FORMAT")))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration error", "error", err)
		return 2
	}
	logger = newLogger(cfg.LogLevel, cfg.LogFormat)

	logger.Info("redirurl-populator starting", append([]any{"version", version}, cfg.LogAttrs()...)...)
	if cfg.InsecureSkipTLSVerify {
		logger.Warn("TLS certificate verification is DISABLED for the remote cluster (set KUBE_INSECURE_SKIP_TLS_VERIFY=false and provide KUBE_CA_CERT[_FILE] to enable it)")
	}

	client, err := kube.NewDynamicClient(cfg, "redirurl-populator/"+version)
	if err != nil {
		logger.Error("failed to build Kubernetes client", "error", err)
		return 1
	}
	ri := kube.OAuthClientInterface(client, cfg.OAuthClientNamespace)

	rec := reconciler.New(ri, cfg.OAuthClientName, cfg.RedirectURIs, cfg.DryRun, 2*cfg.RequestTimeout, logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// One-shot mode: reconcile once and exit with a status that reflects it.
	// Useful as a Job or an init container.
	if cfg.RunOnce {
		if err := rec.RunOnce(ctx); err != nil {
			logger.Error("run-once reconcile failed", "error", err)
			return 1
		}
		logger.Info("run-once reconcile completed")
		return 0
	}

	var healthSrv *http.Server
	if cfg.HealthAddr != "" {
		healthSrv = startHealthServer(cfg.HealthAddr, rec, logger)
	}

	rec.Run(ctx, cfg.ReconcileInterval)

	if healthSrv != nil {
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = healthSrv.Shutdown(sctx)
	}
	logger.Info("stopped")
	return 0
}

// startHealthServer exposes liveness (/healthz) and readiness (/readyz)
// endpoints. Readiness turns green only after a successful reconcile.
func startHealthServer(addr string, rec *reconciler.Reconciler, logger *slog.Logger) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		h := rec.Health()
		switch {
		case h.Reconciles == 0:
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("no reconcile completed yet"))
		case h.LastErr != nil:
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = fmt.Fprintf(w, "last reconcile failed: %v", h.LastErr)
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		}
	})

	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		logger.Info("health server listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("health server error", "error", err)
		}
	}()
	return srv
}

func newLogger(level, format string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: lvl}
	var h slog.Handler
	if format == "text" {
		h = slog.NewTextHandler(os.Stderr, opts)
	} else {
		h = slog.NewJSONHandler(os.Stderr, opts)
	}
	return slog.New(h)
}
