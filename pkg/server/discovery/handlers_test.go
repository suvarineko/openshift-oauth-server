package discovery

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	osinv1 "github.com/openshift/api/osin/v1"
	"github.com/openshift/library-go/pkg/oauth/oauthdiscovery"
)

func TestHandleOAuthDiscovery(t *testing.T) {
	config := &osinv1.OAuthConfig{
		MasterPublicURL: "https://oauth-console.apps.k8s.r4c-test.solution.sbt",
	}

	server := NewDiscoveryServer(config)

	testCases := []struct {
		name           string
		method         string
		expectedStatus int
		expectedJSON   bool
	}{
		{
			name:           "GET request",
			method:         "GET",
			expectedStatus: http.StatusOK,
			expectedJSON:   true,
		},
		{
			name:           "POST request",
			method:         "POST",
			expectedStatus: http.StatusMethodNotAllowed,
			expectedJSON:   false,
		},
		{
			name:           "PUT request",
			method:         "PUT",
			expectedStatus: http.StatusMethodNotAllowed,
			expectedJSON:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, WellKnownOAuthPath, nil)
			rec := httptest.NewRecorder()

			server.HandleOAuthDiscovery(rec, req)

			if rec.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, rec.Code)
			}

			if tc.expectedJSON {
				// Check Content-Type header
				contentType := rec.Header().Get("Content-Type")
				if contentType != "application/json" {
					t.Errorf("Expected Content-Type application/json, got %s", contentType)
				}

				// Check Cache-Control header
				cacheControl := rec.Header().Get("Cache-Control")
				if cacheControl == "" {
					t.Error("Expected Cache-Control header to be set")
				}

				// Parse and validate JSON response
				var metadata oauthdiscovery.OauthAuthorizationServerMetadata
				if err := json.Unmarshal(rec.Body.Bytes(), &metadata); err != nil {
					t.Fatalf("Failed to parse JSON response: %v", err)
				}

				// Validate required fields match expected values from the example
				if metadata.Issuer != config.MasterPublicURL {
					t.Errorf("Expected issuer %s, got %s", config.MasterPublicURL, metadata.Issuer)
				}

				expectedAuthzEndpoint := config.MasterPublicURL + "/oauth/authorize"
				if metadata.AuthorizationEndpoint != expectedAuthzEndpoint {
					t.Errorf("Expected authorization endpoint %s, got %s", expectedAuthzEndpoint, metadata.AuthorizationEndpoint)
				}

				expectedTokenEndpoint := config.MasterPublicURL + "/oauth/token"
				if metadata.TokenEndpoint != expectedTokenEndpoint {
					t.Errorf("Expected token endpoint %s, got %s", expectedTokenEndpoint, metadata.TokenEndpoint)
				}

				// Validate arrays contain expected values from the example
				expectedGrantTypes := []string{"authorization_code", "implicit"}
				if !containsAll(metadata.GrantTypesSupported, expectedGrantTypes) {
					t.Errorf("Grant types should contain %v, got %v", expectedGrantTypes, metadata.GrantTypesSupported)
				}

				expectedResponseTypes := []string{"code", "token"}
				if !containsAll(metadata.ResponseTypesSupported, expectedResponseTypes) {
					t.Errorf("Response types should contain %v, got %v", expectedResponseTypes, metadata.ResponseTypesSupported)
				}

				expectedCodeChallengeMethods := []string{"plain", "S256"}
				if !containsAll(metadata.CodeChallengeMethodsSupported, expectedCodeChallengeMethods) {
					t.Errorf("Code challenge methods should contain %v, got %v", expectedCodeChallengeMethods, metadata.CodeChallengeMethodsSupported)
				}

				// Validate scopes contain expected values from the example
				expectedScopes := []string{"user:check-access", "user:full", "user:info", "user:list-projects", "user:list-scoped-projects"}
				if !containsAll(metadata.ScopesSupported, expectedScopes) {
					t.Errorf("Scopes should contain %v, got %v", expectedScopes, metadata.ScopesSupported)
				}
			}
		})
	}
}

func TestHandleOIDCDiscovery(t *testing.T) {
	config := &osinv1.OAuthConfig{
		MasterPublicURL: "https://oauth-console.apps.k8s.r4c-test.solution.sbt",
	}

	server := NewDiscoveryServer(config)

	testCases := []struct {
		name           string
		method         string
		expectedStatus int
		expectedJSON   bool
	}{
		{
			name:           "GET request",
			method:         "GET",
			expectedStatus: http.StatusOK,
			expectedJSON:   true,
		},
		{
			name:           "POST request",
			method:         "POST",
			expectedStatus: http.StatusMethodNotAllowed,
			expectedJSON:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, WellKnownOIDCPath, nil)
			rec := httptest.NewRecorder()

			server.HandleOIDCDiscovery(rec, req)

			if rec.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, rec.Code)
			}

			if tc.expectedJSON {
				// Check Content-Type header
				contentType := rec.Header().Get("Content-Type")
				if contentType != "application/json" {
					t.Errorf("Expected Content-Type application/json, got %s", contentType)
				}

				// Parse and validate JSON response
				var metadata OIDCProviderMetadata
				if err := json.Unmarshal(rec.Body.Bytes(), &metadata); err != nil {
					t.Fatalf("Failed to parse JSON response: %v", err)
				}

				// Validate required OIDC fields
				if metadata.Issuer != config.MasterPublicURL {
					t.Errorf("Expected issuer %s, got %s", config.MasterPublicURL, metadata.Issuer)
				}

				if len(metadata.ResponseTypesSupported) == 0 {
					t.Error("ResponseTypesSupported should not be empty")
				}

				if len(metadata.SubjectTypesSupported) == 0 {
					t.Error("SubjectTypesSupported should not be empty")
				}

				if len(metadata.IDTokenSigningAlgValuesSupported) == 0 {
					t.Error("IDTokenSigningAlgValuesSupported should not be empty")
				}

				// Validate JWKS URI
				expectedJWKSURI := config.MasterPublicURL + "/.well-known/jwks.json"
				if metadata.JwksURI != expectedJWKSURI {
					t.Errorf("Expected JWKS URI %s, got %s", expectedJWKSURI, metadata.JwksURI)
				}
			}
		})
	}
}

func TestHandleJWKS(t *testing.T) {
	config := &osinv1.OAuthConfig{
		MasterPublicURL: "https://oauth-console.apps.k8s.r4c-test.solution.sbt",
	}

	server := NewDiscoveryServer(config)

	testCases := []struct {
		name           string
		method         string
		expectedStatus int
		expectedJSON   bool
	}{
		{
			name:           "GET request",
			method:         "GET",
			expectedStatus: http.StatusOK,
			expectedJSON:   true,
		},
		{
			name:           "DELETE request",
			method:         "DELETE",
			expectedStatus: http.StatusMethodNotAllowed,
			expectedJSON:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, WellKnownJWKSPath, nil)
			rec := httptest.NewRecorder()

			server.HandleJWKS(rec, req)

			if rec.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, rec.Code)
			}

			if tc.expectedJSON {
				// Check Content-Type header
				contentType := rec.Header().Get("Content-Type")
				if contentType != "application/json" {
					t.Errorf("Expected Content-Type application/json, got %s", contentType)
				}

				// Parse and validate JSON response
				var jwks map[string]interface{}
				if err := json.Unmarshal(rec.Body.Bytes(), &jwks); err != nil {
					t.Fatalf("Failed to parse JSON response: %v", err)
				}

				// Validate JWKS structure
				if _, ok := jwks["keys"]; !ok {
					t.Error("JWKS response should contain 'keys' field")
				}
			}
		})
	}
}

func TestEndpointIntegration(t *testing.T) {
	config := &osinv1.OAuthConfig{
		MasterPublicURL: "https://oauth-console.apps.k8s.r4c-test.solution.sbt",
	}

	// Create a test server with all discovery endpoints
	mux := http.NewServeMux()
	discoveryServer := NewDiscoveryServer(config)
	discoveryEndpoints := NewDiscoveryEndpoints(discoveryServer)
	discoveryEndpoints.Install(mux, "")

	server := httptest.NewServer(mux)
	defer server.Close()

	testCases := []struct {
		path           string
		expectedStatus int
	}{
		{WellKnownOAuthPath, http.StatusOK},
		{WellKnownOIDCPath, http.StatusOK},
		{WellKnownJWKSPath, http.StatusOK},
		{"/nonexistent", http.StatusNotFound},
	}

	for _, tc := range testCases {
		t.Run(tc.path, func(t *testing.T) {
			resp, err := http.Get(server.URL + tc.path)
			if err != nil {
				t.Fatalf("Failed to make request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.expectedStatus {
				t.Errorf("Expected status %d for path %s, got %d", tc.expectedStatus, tc.path, resp.StatusCode)
			}

			if tc.expectedStatus == http.StatusOK {
				contentType := resp.Header.Get("Content-Type")
				if contentType != "application/json" {
					t.Errorf("Expected Content-Type application/json for path %s, got %s", tc.path, contentType)
				}
			}
		})
	}
}

// Helper function to check if all expected items are in the actual slice
func containsAll(actual, expected []string) bool {
	actualSet := make(map[string]bool)
	for _, item := range actual {
		actualSet[item] = true
	}

	for _, item := range expected {
		if !actualSet[item] {
			return false
		}
	}

	return true
}