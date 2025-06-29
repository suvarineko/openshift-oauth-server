package discovery

import (
	oauthserver "github.com/openshift/oauth-server/pkg"
)

const (
	// WellKnownOAuthPath is the standard path for OAuth 2.0 Authorization Server Metadata
	// as defined in RFC 8414: https://tools.ietf.org/html/rfc8414#section-3
	WellKnownOAuthPath = "/.well-known/oauth-authorization-server"
	
	// WellKnownOIDCPath is the standard path for OpenID Connect Provider Configuration
	// as defined in OpenID Connect Discovery 1.0: https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderConfig
	WellKnownOIDCPath = "/.well-known/openid-configuration"
	
	// WellKnownJWKSPath is the standard path for JSON Web Key Set
	// as referenced in OpenID Connect Discovery 1.0 and RFC 7517
	WellKnownJWKSPath = "/.well-known/jwks.json"
)

// discoveryEndpoints implements the oauthserver.Endpoints interface
// to integrate with the existing OAuth server mux installation pattern
type discoveryEndpoints struct {
	server *DiscoveryServer
}

// NewDiscoveryEndpoints creates a new discoveryEndpoints instance
func NewDiscoveryEndpoints(server *DiscoveryServer) oauthserver.Endpoints {
	return &discoveryEndpoints{
		server: server,
	}
}

// Install registers the discovery endpoints with the given mux
// The prefix parameter is ignored as discovery endpoints must be at well-known paths
func (d *discoveryEndpoints) Install(mux oauthserver.Mux, prefix string) {
	// Install OAuth 2.0 Authorization Server Metadata endpoint
	mux.HandleFunc(WellKnownOAuthPath, d.server.HandleOAuthDiscovery)
	
	// Install OpenID Connect Provider Configuration endpoint
	mux.HandleFunc(WellKnownOIDCPath, d.server.HandleOIDCDiscovery)
	
	// Install JSON Web Key Set endpoint
	mux.HandleFunc(WellKnownJWKSPath, d.server.HandleJWKS)
}