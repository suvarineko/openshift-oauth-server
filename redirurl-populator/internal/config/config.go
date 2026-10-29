// Package config loads and validates the runtime configuration of the
// redirurl-populator from environment variables.
package config

import (
	"crypto/x509"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the fully-resolved runtime configuration. Every field is sourced
// from an environment variable; see Load for the mapping.
type Config struct {
	// Connection to the remote (central) cluster hosting the OAuthClient.
	APIServer             string        // KUBE_API_SERVER
	Token                 string        // KUBE_TOKEN or KUBE_TOKEN_FILE
	InsecureSkipTLSVerify bool          // KUBE_INSECURE_SKIP_TLS_VERIFY (default true)
	CACert                []byte        // KUBE_CA_CERT / KUBE_CA_CERT_FILE (PEM, optional)
	RequestTimeout        time.Duration // REQUEST_TIMEOUT (default 30s)

	// Target OAuthClient.
	OAuthClientName      string // OAUTHCLIENT_NAME
	OAuthClientNamespace string // OAUTHCLIENT_NAMESPACE (empty => cluster-scoped, the OpenShift default)

	// Desired redirect URIs that must be present on the OAuthClient.
	RedirectURIs []string // REDIRECT_URIS

	// Behaviour.
	ReconcileInterval time.Duration // RECONCILE_INTERVAL (default 60s)
	RunOnce           bool          // RUN_ONCE (default false)
	DryRun            bool          // DRY_RUN (default false)

	// Observability.
	HealthAddr string // HEALTH_ADDR (default ":8080", empty disables)
	LogLevel   string // LOG_LEVEL (debug|info|warn|error, default info)
	LogFormat  string // LOG_FORMAT (json|text, default json)
}

// Load reads the configuration from the environment, applies defaults and
// validates it. All problems are reported together rather than one at a time.
func Load() (*Config, error) {
	c := &Config{
		APIServer:            strings.TrimSpace(os.Getenv("KUBE_API_SERVER")),
		OAuthClientName:      strings.TrimSpace(os.Getenv("OAUTHCLIENT_NAME")),
		OAuthClientNamespace: strings.TrimSpace(os.Getenv("OAUTHCLIENT_NAMESPACE")),
		RedirectURIs:         splitAndDedup(os.Getenv("REDIRECT_URIS")),
		HealthAddr:           getEnvDefault("HEALTH_ADDR", ":8080"),
		LogLevel:             strings.ToLower(getEnvDefault("LOG_LEVEL", "info")),
		LogFormat:            strings.ToLower(getEnvDefault("LOG_FORMAT", "json")),
	}

	var errs []string
	collect := func(err error) {
		if err != nil {
			errs = append(errs, err.Error())
		}
	}

	var err error
	if c.InsecureSkipTLSVerify, err = getEnvBool("KUBE_INSECURE_SKIP_TLS_VERIFY", true); err != nil {
		collect(err)
	}
	if c.RunOnce, err = getEnvBool("RUN_ONCE", false); err != nil {
		collect(err)
	}
	if c.DryRun, err = getEnvBool("DRY_RUN", false); err != nil {
		collect(err)
	}
	if c.ReconcileInterval, err = getEnvDuration("RECONCILE_INTERVAL", 60*time.Second); err != nil {
		collect(err)
	}
	if c.RequestTimeout, err = getEnvDuration("REQUEST_TIMEOUT", 30*time.Second); err != nil {
		collect(err)
	}

	token, err := loadToken()
	collect(err)
	c.Token = token

	ca, err := loadCACert()
	collect(err)
	c.CACert = ca

	errs = append(errs, c.validate()...)

	if len(errs) > 0 {
		return nil, fmt.Errorf("invalid configuration:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return c, nil
}

func (c *Config) validate() []string {
	var errs []string

	if c.APIServer == "" {
		errs = append(errs, "KUBE_API_SERVER is required (e.g. https://api.central.example.com:6443)")
	} else if u, err := url.Parse(c.APIServer); err != nil || u.Host == "" {
		errs = append(errs, fmt.Sprintf("KUBE_API_SERVER is not a valid URL: %q", c.APIServer))
	} else if u.Scheme != "https" && u.Scheme != "http" {
		errs = append(errs, fmt.Sprintf("KUBE_API_SERVER must use http or https scheme: %q", c.APIServer))
	}

	if c.Token == "" {
		errs = append(errs, "a bearer token is required: set KUBE_TOKEN or KUBE_TOKEN_FILE")
	}

	if c.OAuthClientName == "" {
		errs = append(errs, "OAUTHCLIENT_NAME is required")
	}

	if len(c.RedirectURIs) == 0 {
		errs = append(errs, "REDIRECT_URIS is required (comma/space/newline separated list of URLs)")
	} else {
		for _, u := range c.RedirectURIs {
			if pu, err := url.Parse(u); err != nil || pu.Scheme == "" || pu.Host == "" {
				errs = append(errs, fmt.Sprintf("REDIRECT_URIS contains an invalid absolute URL: %q", u))
			}
		}
	}

	if c.ReconcileInterval <= 0 {
		errs = append(errs, "RECONCILE_INTERVAL must be a positive duration")
	}
	if c.RequestTimeout <= 0 {
		errs = append(errs, "REQUEST_TIMEOUT must be a positive duration")
	}

	if !c.InsecureSkipTLSVerify && len(c.CACert) > 0 {
		if pool := x509.NewCertPool(); !pool.AppendCertsFromPEM(c.CACert) {
			errs = append(errs, "KUBE_CA_CERT/KUBE_CA_CERT_FILE does not contain a valid PEM certificate")
		}
	}

	if c.LogFormat != "json" && c.LogFormat != "text" {
		errs = append(errs, fmt.Sprintf("LOG_FORMAT must be json or text, got %q", c.LogFormat))
	}

	return errs
}

// LogAttrs returns startup log attributes with the bearer token redacted.
func (c *Config) LogAttrs() []any {
	scope := "cluster-scoped"
	if c.OAuthClientNamespace != "" {
		scope = "namespace=" + c.OAuthClientNamespace
	}
	return []any{
		"apiServer", c.APIServer,
		"oauthClient", c.OAuthClientName,
		"scope", scope,
		"redirectURIs", c.RedirectURIs,
		"insecureSkipTLSVerify", c.InsecureSkipTLSVerify,
		"caProvided", len(c.CACert) > 0,
		"reconcileInterval", c.ReconcileInterval.String(),
		"requestTimeout", c.RequestTimeout.String(),
		"runOnce", c.RunOnce,
		"dryRun", c.DryRun,
		"tokenProvided", c.Token != "",
	}
}

// loadToken resolves the bearer token, preferring KUBE_TOKEN_FILE over the
// inline KUBE_TOKEN so tokens can be mounted and rotated as files.
func loadToken() (string, error) {
	if f := strings.TrimSpace(os.Getenv("KUBE_TOKEN_FILE")); f != "" {
		b, err := os.ReadFile(f)
		if err != nil {
			return "", fmt.Errorf("reading KUBE_TOKEN_FILE %q: %w", f, err)
		}
		return strings.TrimSpace(string(b)), nil
	}
	return strings.TrimSpace(os.Getenv("KUBE_TOKEN")), nil
}

// loadCACert resolves an optional CA bundle, preferring the file form.
func loadCACert() ([]byte, error) {
	if f := strings.TrimSpace(os.Getenv("KUBE_CA_CERT_FILE")); f != "" {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("reading KUBE_CA_CERT_FILE %q: %w", f, err)
		}
		return b, nil
	}
	if inline := os.Getenv("KUBE_CA_CERT"); strings.TrimSpace(inline) != "" {
		return []byte(inline), nil
	}
	return nil, nil
}

// splitAndDedup splits a list on commas, semicolons and any whitespace, trims
// each element and removes exact duplicates while preserving first-seen order.
func splitAndDedup(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})
	seen := make(map[string]struct{}, len(fields))
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		if _, ok := seen[f]; ok {
			continue
		}
		seen[f] = struct{}{}
		out = append(out, f)
	}
	return out
}

func getEnvDefault(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return v
	}
	return def
}

func getEnvBool(key string, def bool) (bool, error) {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return def, nil
	}
	b, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		return def, fmt.Errorf("%s is not a valid boolean (true/false): %q", key, v)
	}
	return b, nil
}

func getEnvDuration(key string, def time.Duration) (time.Duration, error) {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return def, nil
	}
	d, err := time.ParseDuration(strings.TrimSpace(v))
	if err != nil {
		return def, fmt.Errorf("%s is not a valid duration such as 60s or 5m: %q", key, v)
	}
	return d, nil
}
