package discovery

// OIDCProviderMetadata holds OpenID Connect Provider Configuration Information
// as defined in OpenID Connect Discovery 1.0 specification
// https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata
type OIDCProviderMetadata struct {
	// REQUIRED. URL using the https scheme with no query or fragment component that the OP asserts as its Issuer Identifier.
	Issuer string `json:"issuer"`

	// REQUIRED. URL of the OP's OAuth 2.0 Authorization Endpoint
	AuthorizationEndpoint string `json:"authorization_endpoint"`

	// URL of the OP's OAuth 2.0 Token Endpoint
	TokenEndpoint string `json:"token_endpoint,omitempty"`

	// RECOMMENDED. URL of the OP's UserInfo Endpoint
	UserinfoEndpoint string `json:"userinfo_endpoint,omitempty"`

	// REQUIRED. URL of the OP's JSON Web Key Set document
	JwksURI string `json:"jwks_uri"`

	// RECOMMENDED. URL of the OP's Dynamic Client Registration Endpoint
	RegistrationEndpoint string `json:"registration_endpoint,omitempty"`

	// OPTIONAL. JSON array containing a list of the OAuth 2.0 scope values that this server supports
	ScopesSupported []string `json:"scopes_supported,omitempty"`

	// REQUIRED. JSON array containing a list of the OAuth 2.0 response_type values that this OP supports
	ResponseTypesSupported []string `json:"response_types_supported"`

	// OPTIONAL. JSON array containing a list of the OAuth 2.0 response_mode values that this OP supports
	ResponseModesSupported []string `json:"response_modes_supported,omitempty"`

	// OPTIONAL. JSON array containing a list of the OAuth 2.0 Grant Type values that this OP supports
	GrantTypesSupported []string `json:"grant_types_supported,omitempty"`

	// OPTIONAL. JSON array containing a list of the Authentication Context Class References that this OP supports
	AcrValuesSupported []string `json:"acr_values_supported,omitempty"`

	// REQUIRED. JSON array containing a list of the Subject Identifier types that this OP supports
	SubjectTypesSupported []string `json:"subject_types_supported"`

	// REQUIRED. JSON array containing a list of the JWS signing algorithms (alg values) supported by the OP for the ID Token
	IDTokenSigningAlgValuesSupported []string `json:"id_token_signing_alg_values_supported"`

	// OPTIONAL. JSON array containing a list of the JWE encryption algorithms (alg values) supported by the OP for the ID Token
	IDTokenEncryptionAlgValuesSupported []string `json:"id_token_encryption_alg_values_supported,omitempty"`

	// OPTIONAL. JSON array containing a list of the JWE encryption algorithms (enc values) supported by the OP for the ID Token
	IDTokenEncryptionEncValuesSupported []string `json:"id_token_encryption_enc_values_supported,omitempty"`

	// OPTIONAL. JSON array containing a list of the JWS signing algorithms (alg values) supported by the UserInfo Endpoint
	UserinfoSigningAlgValuesSupported []string `json:"userinfo_signing_alg_values_supported,omitempty"`

	// OPTIONAL. JSON array containing a list of the JWE encryption algorithms (alg values) supported by the UserInfo Endpoint
	UserinfoEncryptionAlgValuesSupported []string `json:"userinfo_encryption_alg_values_supported,omitempty"`

	// OPTIONAL. JSON array containing a list of the JWE encryption algorithms (enc values) supported by the UserInfo Endpoint
	UserinfoEncryptionEncValuesSupported []string `json:"userinfo_encryption_enc_values_supported,omitempty"`

	// OPTIONAL. JSON array containing a list of the JWS signing algorithms (alg values) supported by the OP for Request Objects
	RequestObjectSigningAlgValuesSupported []string `json:"request_object_signing_alg_values_supported,omitempty"`

	// OPTIONAL. JSON array containing a list of the JWE encryption algorithms (alg values) supported by the OP for Request Objects
	RequestObjectEncryptionAlgValuesSupported []string `json:"request_object_encryption_alg_values_supported,omitempty"`

	// OPTIONAL. JSON array containing a list of the JWE encryption algorithms (enc values) supported by the OP for Request Objects
	RequestObjectEncryptionEncValuesSupported []string `json:"request_object_encryption_enc_values_supported,omitempty"`

	// OPTIONAL. JSON array containing a list of the display parameter values that the OpenID Provider supports
	DisplayValuesSupported []string `json:"display_values_supported,omitempty"`

	// OPTIONAL. JSON array containing a list of the Claim Types that the OpenID Provider supports
	ClaimTypesSupported []string `json:"claim_types_supported,omitempty"`

	// RECOMMENDED. JSON array containing a list of the Claim Names of the Claims that the OpenID Provider MAY be able to supply values for
	ClaimsSupported []string `json:"claims_supported,omitempty"`

	// OPTIONAL. URL of a page containing human-readable information that developers might want or need to know when using the OpenID Provider
	ServiceDocumentation string `json:"service_documentation,omitempty"`

	// OPTIONAL. Languages and scripts supported for values in Claims being returned
	ClaimsLocalesSupported []string `json:"claims_locales_supported,omitempty"`

	// OPTIONAL. Languages and scripts supported for the user interface
	UILocalesSupported []string `json:"ui_locales_supported,omitempty"`

	// OPTIONAL. Boolean value specifying whether the OP supports use of the claims parameter
	ClaimsParameterSupported bool `json:"claims_parameter_supported,omitempty"`

	// OPTIONAL. Boolean value specifying whether the OP supports use of the request parameter
	RequestParameterSupported bool `json:"request_parameter_supported,omitempty"`

	// OPTIONAL. Boolean value specifying whether the OP supports use of the request_uri parameter
	RequestURIParameterSupported bool `json:"request_uri_parameter_supported,omitempty"`

	// OPTIONAL. Boolean value specifying whether the OP requires any request_uri values used to be pre-registered
	RequireRequestURIRegistration bool `json:"require_request_uri_registration,omitempty"`

	// OPTIONAL. URL that the OpenID Provider provides to the person registering the Client to read about the OP's requirements on how the Relying Party can use the data provided by the OP
	OpPolicyURI string `json:"op_policy_uri,omitempty"`

	// OPTIONAL. URL that the OpenID Provider provides to the person registering the Client to read about OpenID Provider's terms of service
	OpTosURI string `json:"op_tos_uri,omitempty"`

	// OPTIONAL. JSON array containing a list of PKCE code challenge methods supported by this authorization server
	CodeChallengeMethodsSupported []string `json:"code_challenge_methods_supported,omitempty"`
}

// DefaultOAuthScopes returns the default OAuth scopes supported by OpenShift OAuth server
func DefaultOAuthScopes() []string {
	return []string{
		"user:check-access",
		"user:full", 
		"user:info",
		"user:list-projects",
		"user:list-scoped-projects",
	}
}

// DefaultGrantTypes returns the default OAuth 2.0 grant types supported
func DefaultGrantTypes() []string {
	return []string{
		"authorization_code",
		"implicit",
	}
}

// DefaultResponseTypes returns the default OAuth 2.0 response types supported
func DefaultResponseTypes() []string {
	return []string{
		"code",
		"token",
	}
}

// DefaultCodeChallengeMethods returns the supported PKCE code challenge methods
func DefaultCodeChallengeMethods() []string {
	return []string{
		"plain",
		"S256",
	}
}

// DefaultOIDCResponseTypes returns the default OIDC response types supported
func DefaultOIDCResponseTypes() []string {
	return []string{
		"code",
		"token",
		"id_token",
		"code token",
		"code id_token",
		"token id_token",
		"code token id_token",
	}
}

// DefaultSubjectTypes returns the default subject identifier types supported
func DefaultSubjectTypes() []string {
	return []string{
		"public",
	}
}

// DefaultIDTokenSigningAlgorithms returns the default ID token signing algorithms supported
func DefaultIDTokenSigningAlgorithms() []string {
	return []string{
		"RS256",
	}
}

// DefaultClaimsSupported returns the default claims that may be supported in ID tokens
func DefaultClaimsSupported() []string {
	return []string{
		"aud",
		"exp",
		"iat",
		"iss",
		"sub",
		"at_hash",
		"email",
		"email_verified",
		"name",
		"preferred_username",
		"groups",
	}
}