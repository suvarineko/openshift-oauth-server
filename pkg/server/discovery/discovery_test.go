package discovery

import (
	"os"
	"testing"
	"k8s.io/apimachinery/pkg/runtime"

	configv1 "github.com/openshift/api/config/v1"
	osinv1 "github.com/openshift/api/osin/v1"
	"github.com/openshift/library-go/pkg/oauth/oauthdiscovery"
)

func TestNewDiscoveryServer(t *testing.T) {
	config := &osinv1.OAuthConfig{
		MasterPublicURL: "https://oauth-console.apps.k8s.r4c-test.solution.sbt",
	}

	server := NewDiscoveryServer(config)
	if server == nil {
		t.Fatal("NewDiscoveryServer returned nil")
	}

	if server.config != config {
		t.Error("DiscoveryServer config not set correctly")
	}

	// Should have default discovery config with JWKS disabled
	if server.discoveryConfig == nil {
		t.Error("DiscoveryServer discoveryConfig not set")
	}

	if server.discoveryConfig.EnableJWKS {
		t.Error("Expected JWKS to be disabled by default")
	}
}

func TestNewDiscoveryServerWithConfig(t *testing.T) {
	oauthConfig := &osinv1.OAuthConfig{
		MasterPublicURL: "https://oauth-console.apps.k8s.r4c-test.solution.sbt",
	}

	discoveryConfig := &DiscoveryConfig{
		EnableJWKS: true,
	}

	server := NewDiscoveryServerWithConfig(oauthConfig, discoveryConfig)
	if server == nil {
		t.Fatal("NewDiscoveryServerWithConfig returned nil")
	}

	if server.config != oauthConfig {
		t.Error("DiscoveryServer oauth config not set correctly")
	}

	if server.discoveryConfig != discoveryConfig {
		t.Error("DiscoveryServer discovery config not set correctly")
	}

	if !server.discoveryConfig.EnableJWKS {
		t.Error("Expected JWKS to be enabled")
	}
}

func TestBuildOAuthMetadata(t *testing.T) {
	testCases := []struct {
		name     string
		config   *osinv1.OAuthConfig
		expected func(*oauthdiscovery.OauthAuthorizationServerMetadata) bool
	}{
		{
			name: "basic configuration",
			config: &osinv1.OAuthConfig{
				MasterPublicURL: "https://oauth-console.apps.k8s.r4c-test.solution.sbt",
			},
			expected: func(metadata *oauthdiscovery.OauthAuthorizationServerMetadata) bool {
				return metadata.Issuer == "https://oauth-console.apps.k8s.r4c-test.solution.sbt" &&
					metadata.AuthorizationEndpoint == "https://oauth-console.apps.k8s.r4c-test.solution.sbt/oauth/authorize" &&
					metadata.TokenEndpoint == "https://oauth-console.apps.k8s.r4c-test.solution.sbt/oauth/token" &&
					len(metadata.ScopesSupported) > 0 &&
					len(metadata.ResponseTypesSupported) > 0 &&
					len(metadata.GrantTypesSupported) > 0 &&
					len(metadata.CodeChallengeMethodsSupported) > 0
			},
		},
		{
			name: "with trailing slash in URL",
			config: &osinv1.OAuthConfig{
				MasterPublicURL: "https://oauth-console.apps.k8s.r4c-test.solution.sbt/",
			},
			expected: func(metadata *oauthdiscovery.OauthAuthorizationServerMetadata) bool {
				return metadata.Issuer == "https://oauth-console.apps.k8s.r4c-test.solution.sbt"
			},
		},
		{
			name: "with query parameters in URL",
			config: &osinv1.OAuthConfig{
				MasterPublicURL: "https://oauth-console.apps.k8s.r4c-test.solution.sbt?param=value",
			},
			expected: func(metadata *oauthdiscovery.OauthAuthorizationServerMetadata) bool {
				return metadata.Issuer == "https://oauth-console.apps.k8s.r4c-test.solution.sbt"
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := NewDiscoveryServer(tc.config)
			metadata := server.BuildOAuthMetadata()

			if metadata == nil {
				t.Fatal("BuildOAuthMetadata returned nil")
			}

			if !tc.expected(metadata) {
				t.Errorf("Expected condition failed for test case: %s", tc.name)
			}
		})
	}
}

func TestBuildOIDCMetadata(t *testing.T) {
	testCases := []struct {
		name        string
		enableJWKS  bool
		expectJWKS  bool
	}{
		{
			name:        "JWKS disabled",
			enableJWKS:  false,
			expectJWKS:  false,
		},
		{
			name:        "JWKS enabled",
			enableJWKS:  true,
			expectJWKS:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := &osinv1.OAuthConfig{
				MasterPublicURL: "https://oauth-console.apps.k8s.r4c-test.solution.sbt",
			}

			discoveryConfig := &DiscoveryConfig{
				EnableJWKS: tc.enableJWKS,
			}

			server := NewDiscoveryServerWithConfig(config, discoveryConfig)
			metadata := server.BuildOIDCMetadata()

			if metadata == nil {
				t.Fatal("BuildOIDCMetadata returned nil")
			}

			// Test required OIDC fields
			if metadata.Issuer != "https://oauth-console.apps.k8s.r4c-test.solution.sbt" {
				t.Errorf("Expected issuer to be %s, got %s", config.MasterPublicURL, metadata.Issuer)
			}

			if metadata.AuthorizationEndpoint != "https://oauth-console.apps.k8s.r4c-test.solution.sbt/oauth/authorize" {
				t.Errorf("Expected authorization endpoint to be correct, got %s", metadata.AuthorizationEndpoint)
			}

			if metadata.TokenEndpoint != "https://oauth-console.apps.k8s.r4c-test.solution.sbt/oauth/token" {
				t.Errorf("Expected token endpoint to be correct, got %s", metadata.TokenEndpoint)
			}

			// Test JWKS URI based on configuration
			if tc.expectJWKS {
				expectedJWKSURI := "https://oauth-console.apps.k8s.r4c-test.solution.sbt/.well-known/jwks.json"
				if metadata.JwksURI != expectedJWKSURI {
					t.Errorf("Expected JWKS URI to be %s, got %s", expectedJWKSURI, metadata.JwksURI)
				}
			} else {
				if metadata.JwksURI != "" {
					t.Errorf("Expected JWKS URI to be empty when disabled, got %s", metadata.JwksURI)
				}
			}

			// Test required arrays are not empty
			if len(metadata.ResponseTypesSupported) == 0 {
				t.Error("ResponseTypesSupported should not be empty")
			}

			if len(metadata.SubjectTypesSupported) == 0 {
				t.Error("SubjectTypesSupported should not be empty")
			}

			if len(metadata.IDTokenSigningAlgValuesSupported) == 0 {
				t.Error("IDTokenSigningAlgValuesSupported should not be empty")
			}
		})
	}
}

func TestGetSupportedScopes(t *testing.T) {
	testCases := []struct {
		name                string
		config              *osinv1.OAuthConfig
		expectOpenIDInScope bool
	}{
		{
			name: "no identity providers",
			config: &osinv1.OAuthConfig{
				MasterPublicURL:   "https://oauth-console.apps.k8s.r4c-test.solution.sbt",
				IdentityProviders: []osinv1.IdentityProvider{},
			},
			expectOpenIDInScope: false,
		},
		{
			name: "with OpenID Connect provider",
			config: &osinv1.OAuthConfig{
				MasterPublicURL: "https://oauth-console.apps.k8s.r4c-test.solution.sbt",
				IdentityProviders: []osinv1.IdentityProvider{
					{
						Name: "oidc",
						Provider: runtime.RawExtension{
							Object: &osinv1.OpenIDIdentityProvider{
								ClientID:     "client-id",
								ClientSecret: configv1.StringSource{
									StringSourceSpec: configv1.StringSourceSpec{
										Value: "secret",
									},
								},
								URLs: osinv1.OpenIDURLs{
									Authorize: "https://provider.com/auth",
									Token:     "https://provider.com/token",
								},
								Claims: osinv1.OpenIDClaims{
									ID: []string{"sub"},
								},
							},
						},
					},
				},
			},
			expectOpenIDInScope: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := NewDiscoveryServer(tc.config)
			scopes := server.getSupportedScopes()

			if len(scopes) == 0 {
				t.Error("getSupportedScopes should return at least some scopes")
			}

			hasOpenID := false
			for _, scope := range scopes {
				if scope == "openid" {
					hasOpenID = true
					break
				}
			}

			if hasOpenID != tc.expectOpenIDInScope {
				t.Errorf("Expected openid scope presence: %v, got: %v", tc.expectOpenIDInScope, hasOpenID)
			}
		})
	}
}

func TestGetIssuerURL(t *testing.T) {
	testCases := []struct {
		name        string
		inputURL    string
		expectedURL string
	}{
		{
			name:        "clean URL",
			inputURL:    "https://oauth-console.apps.k8s.r4c-test.solution.sbt",
			expectedURL: "https://oauth-console.apps.k8s.r4c-test.solution.sbt",
		},
		{
			name:        "URL with trailing slash",
			inputURL:    "https://oauth-console.apps.k8s.r4c-test.solution.sbt/",
			expectedURL: "https://oauth-console.apps.k8s.r4c-test.solution.sbt",
		},
		{
			name:        "URL with query parameters",
			inputURL:    "https://oauth-console.apps.k8s.r4c-test.solution.sbt?param=value",
			expectedURL: "https://oauth-console.apps.k8s.r4c-test.solution.sbt",
		},
		{
			name:        "URL with fragment",
			inputURL:    "https://oauth-console.apps.k8s.r4c-test.solution.sbt#fragment",
			expectedURL: "https://oauth-console.apps.k8s.r4c-test.solution.sbt",
		},
		{
			name:        "URL with path",
			inputURL:    "https://oauth-console.apps.k8s.r4c-test.solution.sbt/path",
			expectedURL: "https://oauth-console.apps.k8s.r4c-test.solution.sbt/path",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := &osinv1.OAuthConfig{
				MasterPublicURL: tc.inputURL,
			}

			server := NewDiscoveryServer(config)
			issuer := server.getIssuerURL()

			if issuer != tc.expectedURL {
				t.Errorf("Expected issuer URL %s, got %s", tc.expectedURL, issuer)
			}
		})
	}
}

func TestDefaultValues(t *testing.T) {
	// Test default OAuth scopes
	scopes := DefaultOAuthScopes()
	expectedScopes := []string{"user:check-access", "user:full", "user:info", "user:list-projects", "user:list-scoped-projects"}
	if len(scopes) != len(expectedScopes) {
		t.Errorf("Expected %d default scopes, got %d", len(expectedScopes), len(scopes))
	}

	// Test default grant types
	grantTypes := DefaultGrantTypes()
	expectedGrantTypes := []string{"authorization_code", "implicit"}
	if len(grantTypes) != len(expectedGrantTypes) {
		t.Errorf("Expected %d default grant types, got %d", len(expectedGrantTypes), len(grantTypes))
	}

	// Test default response types
	responseTypes := DefaultResponseTypes()
	expectedResponseTypes := []string{"code", "token"}
	if len(responseTypes) != len(expectedResponseTypes) {
		t.Errorf("Expected %d default response types, got %d", len(expectedResponseTypes), len(responseTypes))
	}

	// Test code challenge methods
	challengeMethods := DefaultCodeChallengeMethods()
	expectedChallengeMethods := []string{"plain", "S256"}
	if len(challengeMethods) != len(expectedChallengeMethods) {
		t.Errorf("Expected %d default code challenge methods, got %d", len(expectedChallengeMethods), len(challengeMethods))
	}
}

func TestDiscoveryConfig(t *testing.T) {
	t.Run("NewDefaultDiscoveryConfig", func(t *testing.T) {
		config := NewDefaultDiscoveryConfig()
		if config == nil {
			t.Fatal("NewDefaultDiscoveryConfig returned nil")
		}
		
		// JWKS should be disabled by default
		if config.EnableJWKS {
			t.Error("Expected EnableJWKS to be false by default")
		}
	})

	t.Run("NewDiscoveryConfigFromEnv enabled", func(t *testing.T) {
		// Set environment variable
		os.Setenv("OAUTH_DISCOVERY_ENABLE_JWKS", "true")
		defer os.Unsetenv("OAUTH_DISCOVERY_ENABLE_JWKS")

		config := NewDiscoveryConfigFromEnv()
		if config == nil {
			t.Fatal("NewDiscoveryConfigFromEnv returned nil")
		}

		if !config.EnableJWKS {
			t.Error("Expected EnableJWKS to be true when env var is 'true'")
		}
	})

	t.Run("NewDiscoveryConfigFromEnv disabled", func(t *testing.T) {
		// Ensure environment variable is not set or is set to false
		os.Setenv("OAUTH_DISCOVERY_ENABLE_JWKS", "false")
		defer os.Unsetenv("OAUTH_DISCOVERY_ENABLE_JWKS")

		config := NewDiscoveryConfigFromEnv()
		if config == nil {
			t.Fatal("NewDiscoveryConfigFromEnv returned nil")
		}

		if config.EnableJWKS {
			t.Error("Expected EnableJWKS to be false when env var is not 'true'")
		}
	})

	t.Run("NewDiscoveryConfigFromEnv unset", func(t *testing.T) {
		// Ensure environment variable is not set
		os.Unsetenv("OAUTH_DISCOVERY_ENABLE_JWKS")

		config := NewDiscoveryConfigFromEnv()
		if config == nil {
			t.Fatal("NewDiscoveryConfigFromEnv returned nil")
		}

		if config.EnableJWKS {
			t.Error("Expected EnableJWKS to be false when env var is unset")
		}
	})
}