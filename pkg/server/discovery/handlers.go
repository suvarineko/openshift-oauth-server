package discovery

import (
	"encoding/json"
	"net/http"

	"k8s.io/klog/v2"
)

// HandleOAuthDiscovery handles requests to the OAuth 2.0 Authorization Server Metadata endpoint
// Implements RFC 8414: https://tools.ietf.org/html/rfc8414#section-3
func (d *DiscoveryServer) HandleOAuthDiscovery(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Build the OAuth metadata
	metadata := d.BuildOAuthMetadata()

	// Set appropriate headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600") // Cache for 1 hour
	
	// Encode and send the response
	if err := json.NewEncoder(w).Encode(metadata); err != nil {
		klog.Errorf("Failed to encode OAuth discovery metadata: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	klog.V(6).Infof("Served OAuth 2.0 Authorization Server Metadata for issuer: %s", metadata.Issuer)
}

// HandleOIDCDiscovery handles requests to the OpenID Connect Provider Configuration endpoint
// Implements OpenID Connect Discovery 1.0: https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderConfig
func (d *DiscoveryServer) HandleOIDCDiscovery(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Build the OIDC metadata
	metadata := d.BuildOIDCMetadata()

	// Set appropriate headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600") // Cache for 1 hour
	
	// Encode and send the response
	if err := json.NewEncoder(w).Encode(metadata); err != nil {
		klog.Errorf("Failed to encode OIDC discovery metadata: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	klog.V(6).Infof("Served OpenID Connect Provider Configuration for issuer: %s", metadata.Issuer)
}

// HandleJWKS handles requests to the JSON Web Key Set endpoint
// This is a placeholder implementation - actual JWKS would need to be implemented
// based on the server's signing keys
func (d *DiscoveryServer) HandleJWKS(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// For now, return an empty JWKS as OpenShift OAuth server doesn't use JWTs by default
	// In a full implementation, this would return the actual public keys used for signing
	jwks := map[string]interface{}{
		"keys": []interface{}{},
	}

	// Set appropriate headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=86400") // Cache for 24 hours
	
	// Encode and send the response
	if err := json.NewEncoder(w).Encode(jwks); err != nil {
		klog.Errorf("Failed to encode JWKS: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	klog.V(6).Infof("Served JWKS for issuer: %s", d.GetIssuer())
}