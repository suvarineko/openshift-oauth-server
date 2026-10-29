package config

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

// clearEnv sets every environment variable the loader reads to empty so a test
// starts from a known baseline regardless of the host environment.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"KUBE_API_SERVER", "KUBE_TOKEN", "KUBE_TOKEN_FILE",
		"KUBE_INSECURE_SKIP_TLS_VERIFY", "KUBE_CA_CERT", "KUBE_CA_CERT_FILE",
		"REQUEST_TIMEOUT", "OAUTHCLIENT_NAME", "OAUTHCLIENT_NAMESPACE",
		"REDIRECT_URIS", "RECONCILE_INTERVAL", "RUN_ONCE", "DRY_RUN",
		"HEALTH_ADDR", "LOG_LEVEL", "LOG_FORMAT",
	} {
		t.Setenv(k, "")
	}
}

func setValidMinimum(t *testing.T) {
	t.Helper()
	t.Setenv("KUBE_API_SERVER", "https://api.central.example.com:6443")
	t.Setenv("KUBE_TOKEN", "s3cr3t-token")
	t.Setenv("OAUTHCLIENT_NAME", "spoke-1")
	t.Setenv("REDIRECT_URIS", "https://a.example.com,https://b.example.com")
}

func TestLoad_DefaultsAndParsing(t *testing.T) {
	clearEnv(t)
	setValidMinimum(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.InsecureSkipTLSVerify != true {
		t.Errorf("InsecureSkipTLSVerify default = %v, want true", cfg.InsecureSkipTLSVerify)
	}
	if cfg.ReconcileInterval != 60*time.Second {
		t.Errorf("ReconcileInterval default = %v, want 60s", cfg.ReconcileInterval)
	}
	if cfg.RequestTimeout != 30*time.Second {
		t.Errorf("RequestTimeout default = %v, want 30s", cfg.RequestTimeout)
	}
	if cfg.HealthAddr != ":8080" {
		t.Errorf("HealthAddr default = %q, want :8080", cfg.HealthAddr)
	}
	if cfg.LogFormat != "json" || cfg.LogLevel != "info" {
		t.Errorf("log defaults = %q/%q, want json/info", cfg.LogFormat, cfg.LogLevel)
	}
	if cfg.OAuthClientNamespace != "" {
		t.Errorf("OAuthClientNamespace = %q, want empty (cluster-scoped)", cfg.OAuthClientNamespace)
	}
	want := []string{"https://a.example.com", "https://b.example.com"}
	if !reflect.DeepEqual(cfg.RedirectURIs, want) {
		t.Errorf("RedirectURIs = %v, want %v", cfg.RedirectURIs, want)
	}
}

func TestLoad_OverridesAndListSeparators(t *testing.T) {
	clearEnv(t)
	setValidMinimum(t)
	t.Setenv("KUBE_INSECURE_SKIP_TLS_VERIFY", "false")
	t.Setenv("REQUEST_TIMEOUT", "15s")
	t.Setenv("RECONCILE_INTERVAL", "5m")
	t.Setenv("RUN_ONCE", "true")
	t.Setenv("DRY_RUN", "1")
	// Mixed separators and duplicates.
	t.Setenv("REDIRECT_URIS", "https://a.example.com, https://b.example.com\nhttps://a.example.com https://c.example.com")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.InsecureSkipTLSVerify {
		t.Error("InsecureSkipTLSVerify = true, want false")
	}
	if !cfg.RunOnce || !cfg.DryRun {
		t.Errorf("RunOnce/DryRun = %v/%v, want true/true", cfg.RunOnce, cfg.DryRun)
	}
	if cfg.RequestTimeout != 15*time.Second || cfg.ReconcileInterval != 5*time.Minute {
		t.Errorf("durations = %v/%v, want 15s/5m", cfg.RequestTimeout, cfg.ReconcileInterval)
	}
	want := []string{"https://a.example.com", "https://b.example.com", "https://c.example.com"}
	if !reflect.DeepEqual(cfg.RedirectURIs, want) {
		t.Errorf("RedirectURIs = %v, want %v (deduped, order preserved)", cfg.RedirectURIs, want)
	}
}

func TestLoad_TokenFile(t *testing.T) {
	clearEnv(t)
	setValidMinimum(t)
	// KUBE_TOKEN_FILE takes precedence over the inline KUBE_TOKEN.
	dir := t.TempDir()
	f := dir + "/token"
	if err := os.WriteFile(f, []byte("  file-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KUBE_TOKEN", "inline-token")
	t.Setenv("KUBE_TOKEN_FILE", f)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Token != "file-token" {
		t.Errorf("Token = %q, want %q (trimmed file contents)", cfg.Token, "file-token")
	}
}

func TestLoad_MissingRequiredFields(t *testing.T) {
	clearEnv(t)
	// Nothing set: API server, token, name and redirect URIs are all missing.
	_, err := Load()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	for _, must := range []string{"KUBE_API_SERVER", "bearer token", "OAUTHCLIENT_NAME", "REDIRECT_URIS"} {
		if !strings.Contains(err.Error(), must) {
			t.Errorf("error %q does not mention %q", err.Error(), must)
		}
	}
}

func TestLoad_InvalidValues(t *testing.T) {
	clearEnv(t)
	setValidMinimum(t)
	t.Setenv("KUBE_API_SERVER", "not-a-url")
	t.Setenv("REDIRECT_URIS", "https://ok.example.com,::::not-a-url")
	t.Setenv("RECONCILE_INTERVAL", "banana")

	_, err := Load()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	for _, must := range []string{"KUBE_API_SERVER", "REDIRECT_URIS", "RECONCILE_INTERVAL"} {
		if !strings.Contains(err.Error(), must) {
			t.Errorf("error %q does not mention %q", err.Error(), must)
		}
	}
}

func TestSplitAndDedup(t *testing.T) {
	got := splitAndDedup(" a , b\nc\td ;a; b ")
	want := []string{"a", "b", "c", "d"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitAndDedup = %v, want %v", got, want)
	}
	if got := splitAndDedup(""); len(got) != 0 {
		t.Fatalf("splitAndDedup(\"\") = %v, want empty", got)
	}
}
