package reconciler

import (
	"context"
	"io"
	"log/slog"
	"reflect"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

var testGVR = schema.GroupVersionResource{Group: "oauth.openshift.io", Version: "v1", Resource: "oauthclients"}

const testName = "test-client"

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// newFixture builds a Reconciler backed by a fake dynamic client. When existing
// is nil the OAuthClient has no redirectURIs field at all. When createObject is
// false the OAuthClient does not exist on the server.
func newFixture(t *testing.T, desired, existing []string, dryRun, createObject bool) (*Reconciler, dynamic.ResourceInterface) {
	t.Helper()

	scheme := runtime.NewScheme()
	listKinds := map[schema.GroupVersionResource]string{testGVR: "OAuthClientList"}

	var objects []runtime.Object
	if createObject {
		obj := &unstructured.Unstructured{Object: map[string]any{}}
		obj.SetAPIVersion("oauth.openshift.io/v1")
		obj.SetKind("OAuthClient")
		obj.SetName(testName)
		if existing != nil {
			if err := unstructured.SetNestedStringSlice(obj.Object, existing, redirectURIsField); err != nil {
				t.Fatalf("seeding existing redirectURIs: %v", err)
			}
		}
		objects = append(objects, obj)
	}

	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds, objects...)
	ri := client.Resource(testGVR)
	rec := New(ri, testName, desired, dryRun, 5*time.Second, discardLogger())
	return rec, ri
}

func currentURIs(t *testing.T, ri dynamic.ResourceInterface) []string {
	t.Helper()
	obj, err := ri.Get(context.Background(), testName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("getting object: %v", err)
	}
	uris, _, err := unstructured.NestedStringSlice(obj.Object, redirectURIsField)
	if err != nil {
		t.Fatalf("reading redirectURIs: %v", err)
	}
	return uris
}

func TestReconcile_AddsMissing(t *testing.T) {
	desired := []string{"https://a.example.com", "https://b.example.com"}
	rec, ri := newFixture(t, desired, []string{"https://a.example.com"}, false, true)

	added, err := rec.Reconcile(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []string{"https://b.example.com"}; !reflect.DeepEqual(added, want) {
		t.Fatalf("added = %v, want %v", added, want)
	}
	if got, want := currentURIs(t, ri), []string{"https://a.example.com", "https://b.example.com"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("stored redirectURIs = %v, want %v", got, want)
	}
}

func TestReconcile_NoopWhenAllPresent(t *testing.T) {
	desired := []string{"https://a.example.com", "https://b.example.com"}
	existing := []string{"https://a.example.com", "https://b.example.com", "https://extra.example.com"}
	rec, ri := newFixture(t, desired, existing, false, true)

	added, err := rec.Reconcile(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if added != nil {
		t.Fatalf("added = %v, want nil", added)
	}
	// Existing URIs (including unrelated extras) must be preserved untouched.
	if got := currentURIs(t, ri); !reflect.DeepEqual(got, existing) {
		t.Fatalf("stored redirectURIs = %v, want %v", got, existing)
	}
}

func TestReconcile_FieldAbsent(t *testing.T) {
	desired := []string{"https://a.example.com", "https://b.example.com"}
	rec, ri := newFixture(t, desired, nil, false, true)

	added, err := rec.Reconcile(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(added, desired) {
		t.Fatalf("added = %v, want %v", added, desired)
	}
	if got := currentURIs(t, ri); !reflect.DeepEqual(got, desired) {
		t.Fatalf("stored redirectURIs = %v, want %v", got, desired)
	}
}

func TestReconcile_DryRunDoesNotWrite(t *testing.T) {
	desired := []string{"https://a.example.com", "https://b.example.com"}
	existing := []string{"https://a.example.com"}
	rec, ri := newFixture(t, desired, existing, true, true)

	added, err := rec.Reconcile(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := []string{"https://b.example.com"}; !reflect.DeepEqual(added, want) {
		t.Fatalf("added = %v, want %v", added, want)
	}
	if got := currentURIs(t, ri); !reflect.DeepEqual(got, existing) {
		t.Fatalf("dry-run must not mutate: stored = %v, want %v", got, existing)
	}
}

func TestReconcile_NotFound(t *testing.T) {
	desired := []string{"https://a.example.com"}
	rec, _ := newFixture(t, desired, nil, false, false)

	_, err := rec.Reconcile(context.Background())
	if err == nil {
		t.Fatal("expected an error for a missing OAuthClient, got nil")
	}
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected NotFound error, got %v", err)
	}
}

func TestMissingURIs(t *testing.T) {
	tests := []struct {
		name          string
		desired, have []string
		want          []string
	}{
		{"all missing", []string{"a", "b"}, nil, []string{"a", "b"}},
		{"none missing", []string{"a", "b"}, []string{"b", "a"}, nil},
		{"some missing", []string{"a", "b", "c"}, []string{"b"}, []string{"a", "c"}},
		{"desired dupes collapse", []string{"a", "a", "b"}, []string{"b"}, []string{"a"}},
		{"empty desired", nil, []string{"a"}, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := missingURIs(tc.desired, tc.have); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("missingURIs(%v, %v) = %v, want %v", tc.desired, tc.have, got, tc.want)
			}
		})
	}
}
