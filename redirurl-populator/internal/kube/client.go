// Package kube builds a Kubernetes dynamic client for the remote cluster and
// exposes the OAuthClient resource interface.
package kube

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	"github.com/suvarineko/redirurl-populator/internal/config"
)

// OAuthClientGVR is the GroupVersionResource of OpenShift OAuthClients.
// OAuthClient is cluster-scoped (oauth.openshift.io/v1, kind OAuthClient).
var OAuthClientGVR = schema.GroupVersionResource{
	Group:    "oauth.openshift.io",
	Version:  "v1",
	Resource: "oauthclients",
}

// NewDynamicClient builds a dynamic client pointed at the remote cluster
// described by cfg. When cfg.InsecureSkipTLSVerify is true the server
// certificate is not verified and any CA material is ignored.
func NewDynamicClient(cfg *config.Config, userAgent string) (dynamic.Interface, error) {
	rc := &rest.Config{
		Host:        cfg.APIServer,
		BearerToken: cfg.Token,
		UserAgent:   userAgent,
		Timeout:     cfg.RequestTimeout,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: cfg.InsecureSkipTLSVerify,
		},
	}
	// A CA bundle and Insecure are mutually exclusive in client-go, so only
	// attach the CA when we actually verify the server certificate.
	if !cfg.InsecureSkipTLSVerify && len(cfg.CACert) > 0 {
		rc.TLSClientConfig.CAData = cfg.CACert
	}

	client, err := dynamic.NewForConfig(rc)
	if err != nil {
		return nil, fmt.Errorf("building dynamic client: %w", err)
	}
	return client, nil
}

// OAuthClientInterface returns the resource interface for the target
// OAuthClient. An empty namespace selects the cluster-scoped interface, which
// is correct for a standard OpenShift OAuthClient.
func OAuthClientInterface(client dynamic.Interface, namespace string) dynamic.ResourceInterface {
	if namespace == "" {
		return client.Resource(OAuthClientGVR)
	}
	return client.Resource(OAuthClientGVR).Namespace(namespace)
}
