package discovery

import (
	"net/url"
	"path"
	"strings"

	osinv1 "github.com/openshift/api/osin/v1"
	"github.com/openshift/library-go/pkg/oauth/oauthdiscovery"
)

// DiscoveryServer builds OAuth 2.0 and OpenID Connect discovery metadata
type DiscoveryServer struct {
	config          *osinv1.OAuthConfig
	discoveryConfig *DiscoveryConfig
}

// NewDiscoveryServer creates a new discovery server with the given OAuth configuration
func NewDiscoveryServer(config *osinv1.OAuthConfig) *DiscoveryServer {
	return &DiscoveryServer{
		config:          config,
		discoveryConfig: NewDefaultDiscoveryConfig(),
	}
}

// NewDiscoveryServerWithConfig creates a new discovery server with custom discovery configuration
func NewDiscoveryServerWithConfig(config *osinv1.OAuthConfig, discoveryConfig *DiscoveryConfig) *DiscoveryServer {
	return &DiscoveryServer{
		config:          config,
		discoveryConfig: discoveryConfig,
	}
}

// BuildOAuthMetadata constructs OAuth 2.0 Authorization Server Metadata
// as defined in RFC 8414: https://tools.ietf.org/html/rfc8414
func (d *DiscoveryServer) BuildOAuthMetadata() *oauthdiscovery.OauthAuthorizationServerMetadata {
	issuer := d.getIssuerURL()
	
	metadata := &oauthdiscovery.OauthAuthorizationServerMetadata{
		Issuer:                        issuer,
		AuthorizationEndpoint:         d.buildEndpointURL(issuer, oauthdiscovery.OpenShiftOAuthAPIPrefix, oauthdiscovery.AuthorizePath),
		TokenEndpoint:                 d.buildEndpointURL(issuer, oauthdiscovery.OpenShiftOAuthAPIPrefix, oauthdiscovery.TokenPath),
		ScopesSupported:               d.getSupportedScopes(),
		ResponseTypesSupported:        DefaultResponseTypes(),
		GrantTypesSupported:           DefaultGrantTypes(),
		CodeChallengeMethodsSupported: DefaultCodeChallengeMethods(),
	}

	return metadata
}

// BuildOIDCMetadata constructs OpenID Connect Provider Configuration Information
// as defined in OpenID Connect Discovery 1.0: https://openid.net/specs/openid-connect-discovery-1_0.html
func (d *DiscoveryServer) BuildOIDCMetadata() *OIDCProviderMetadata {
	issuer := d.getIssuerURL()
	
	metadata := &OIDCProviderMetadata{
		Issuer:                           issuer,
		AuthorizationEndpoint:            d.buildEndpointURL(issuer, oauthdiscovery.OpenShiftOAuthAPIPrefix, oauthdiscovery.AuthorizePath),
		TokenEndpoint:                    d.buildEndpointURL(issuer, oauthdiscovery.OpenShiftOAuthAPIPrefix, oauthdiscovery.TokenPath),
		UserinfoEndpoint:                 d.buildEndpointURL(issuer, oauthdiscovery.OpenShiftOAuthAPIPrefix, oauthdiscovery.InfoPath),
		JwksURI:                          d.buildEndpointURL(issuer, "/.well-known", "/jwks.json"),
		ScopesSupported:                  d.getSupportedScopes(),
		ResponseTypesSupported:           DefaultOIDCResponseTypes(),
		GrantTypesSupported:              DefaultGrantTypes(),
		SubjectTypesSupported:            DefaultSubjectTypes(),
		IDTokenSigningAlgValuesSupported: DefaultIDTokenSigningAlgorithms(),
		ClaimsSupported:                  DefaultClaimsSupported(),
		CodeChallengeMethodsSupported:    DefaultCodeChallengeMethods(),
		
		// Optional OIDC features
		ClaimsParameterSupported:      false,
		RequestParameterSupported:     false,
		RequestURIParameterSupported:  false,
		RequireRequestURIRegistration: false,
	}

	return metadata
}

// getIssuerURL returns the issuer URL for the OAuth server
func (d *DiscoveryServer) getIssuerURL() string {
	// Use MasterPublicURL as the issuer identifier
	// This must be an HTTPS URL with no query or fragment components
	issuerURL := d.config.MasterPublicURL
	
	// Ensure the URL is clean (no trailing slash, query, or fragment)
	if parsed, err := url.Parse(issuerURL); err == nil {
		parsed.RawQuery = ""
		parsed.Fragment = ""
		parsed.Path = strings.TrimSuffix(parsed.Path, "/")
		issuerURL = parsed.String()
	}
	
	return issuerURL
}

// buildEndpointURL constructs a full endpoint URL from the issuer and path components
func (d *DiscoveryServer) buildEndpointURL(issuer, prefix, endpoint string) string {
	if parsed, err := url.Parse(issuer); err == nil {
		parsed.Path = path.Join(parsed.Path, prefix, endpoint)
		return parsed.String()
	}
	// Fallback to simple concatenation if URL parsing fails
	return issuer + prefix + endpoint
}

// getSupportedScopes returns the OAuth scopes supported by this server
func (d *DiscoveryServer) getSupportedScopes() []string {
	// Start with default OpenShift OAuth scopes
	scopes := DefaultOAuthScopes()
	
	// Add OpenID scope if any OpenID Connect identity providers are configured
	if d.hasOpenIDProviders() {
		scopes = append([]string{"openid"}, scopes...)
	}
	
	return scopes
}

// hasOpenIDProviders checks if any OpenID Connect identity providers are configured
func (d *DiscoveryServer) hasOpenIDProviders() bool {
	for _, provider := range d.config.IdentityProviders {
		if _, isOpenID := provider.Provider.Object.(*osinv1.OpenIDIdentityProvider); isOpenID {
			return true
		}
	}
	return false
}

// GetIssuer returns the issuer URL for external use
func (d *DiscoveryServer) GetIssuer() string {
	return d.getIssuerURL()
}

// GetAuthorizationEndpoint returns the authorization endpoint URL
func (d *DiscoveryServer) GetAuthorizationEndpoint() string {
	issuer := d.getIssuerURL()
	return d.buildEndpointURL(issuer, oauthdiscovery.OpenShiftOAuthAPIPrefix, oauthdiscovery.AuthorizePath)
}

// GetTokenEndpoint returns the token endpoint URL  
func (d *DiscoveryServer) GetTokenEndpoint() string {
	issuer := d.getIssuerURL()
	return d.buildEndpointURL(issuer, oauthdiscovery.OpenShiftOAuthAPIPrefix, oauthdiscovery.TokenPath)
}