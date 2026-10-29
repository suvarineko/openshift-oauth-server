// Package reconciler ensures a single OAuthClient's redirectURIs contain a
// required set of URLs, adding any that are missing.
package reconciler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/util/retry"
)

const redirectURIsField = "redirectURIs"

// Reconciler ensures the desired redirect URIs are present on one OAuthClient.
// It never removes or reorders existing URIs — it only appends missing ones.
type Reconciler struct {
	client  dynamic.ResourceInterface
	name    string
	desired []string
	dryRun  bool
	timeout time.Duration
	log     *slog.Logger

	mu          sync.RWMutex
	reconciles  uint64
	lastErr     error
	lastSuccess time.Time
}

// New returns a Reconciler for the OAuthClient identified by name, reachable
// through client. timeout bounds a single reconcile (a GET plus a possible
// UPDATE, including conflict retries).
func New(client dynamic.ResourceInterface, name string, desired []string, dryRun bool, timeout time.Duration, log *slog.Logger) *Reconciler {
	return &Reconciler{
		client:  client,
		name:    name,
		desired: desired,
		dryRun:  dryRun,
		timeout: timeout,
		log:     log,
	}
}

// Reconcile ensures every desired redirect URI is present on the OAuthClient.
// It returns the URIs it added (or, in dry-run mode, the URIs it would add).
// The returned slice is only meaningful when err is nil. Concurrent writers are
// tolerated via optimistic concurrency: on a conflict the object is re-read and
// the change re-applied.
func (r *Reconciler) Reconcile(ctx context.Context) (added []string, err error) {
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		obj, getErr := r.client.Get(ctx, r.name, metav1.GetOptions{})
		if getErr != nil {
			return getErr
		}

		current, _, fErr := unstructured.NestedStringSlice(obj.Object, redirectURIsField)
		if fErr != nil {
			return fmt.Errorf("reading %s from OAuthClient %q: %w", redirectURIsField, r.name, fErr)
		}

		missing := missingURIs(r.desired, current)
		if len(missing) == 0 {
			added = nil
			return nil
		}

		if r.dryRun {
			added = missing
			return nil
		}

		merged := make([]string, 0, len(current)+len(missing))
		merged = append(merged, current...)
		merged = append(merged, missing...)
		if setErr := unstructured.SetNestedStringSlice(obj.Object, merged, redirectURIsField); setErr != nil {
			return fmt.Errorf("setting %s on OAuthClient %q: %w", redirectURIsField, r.name, setErr)
		}

		if _, upErr := r.client.Update(ctx, obj, metav1.UpdateOptions{}); upErr != nil {
			return upErr
		}
		added = missing
		return nil
	})
	return added, err
}

// reconcileOnce runs a single time-bounded reconcile, records health state and
// logs the outcome. It returns the reconcile error (nil on success).
func (r *Reconciler) reconcileOnce(ctx context.Context) error {
	cctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	added, err := r.Reconcile(cctx)

	r.mu.Lock()
	r.reconciles++
	r.lastErr = err
	if err == nil {
		r.lastSuccess = time.Now()
	}
	r.mu.Unlock()

	switch {
	case apierrors.IsNotFound(err):
		r.log.Error("OAuthClient not found on the remote cluster", "name", r.name, "error", err)
	case err != nil:
		r.log.Error("reconcile failed", "name", r.name, "error", err)
	case len(added) > 0 && r.dryRun:
		r.log.Info("dry-run: redirect URIs would be added", "name", r.name, "wouldAdd", added)
	case len(added) > 0:
		r.log.Info("added redirect URIs to OAuthClient", "name", r.name, "added", added)
	default:
		r.log.Debug("OAuthClient already contains all desired redirect URIs", "name", r.name)
	}
	return err
}

// Run performs an immediate reconcile and then repeats on the given interval
// until ctx is cancelled. Transient errors are logged but do not stop the loop.
func (r *Reconciler) Run(ctx context.Context, interval time.Duration) {
	r.log.Info("starting reconcile loop", "interval", interval.String())
	_ = r.reconcileOnce(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			r.log.Info("stopping reconcile loop")
			return
		case <-ticker.C:
			_ = r.reconcileOnce(ctx)
		}
	}
}

// RunOnce performs exactly one reconcile and returns its error.
func (r *Reconciler) RunOnce(ctx context.Context) error {
	return r.reconcileOnce(ctx)
}

// Health is a snapshot of the reconciler's observable state.
type Health struct {
	Reconciles  uint64
	LastErr     error
	LastSuccess time.Time
}

// Health returns the current health snapshot in a thread-safe way.
func (r *Reconciler) Health() Health {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return Health{Reconciles: r.reconciles, LastErr: r.lastErr, LastSuccess: r.lastSuccess}
}

// missingURIs returns the entries of desired (deduplicated, order preserved)
// that are not already present in current.
func missingURIs(desired, current []string) []string {
	have := make(map[string]struct{}, len(current))
	for _, c := range current {
		have[c] = struct{}{}
	}
	seen := make(map[string]struct{}, len(desired))
	var missing []string
	for _, d := range desired {
		if _, ok := have[d]; ok {
			continue
		}
		if _, ok := seen[d]; ok {
			continue
		}
		seen[d] = struct{}{}
		missing = append(missing, d)
	}
	return missing
}
