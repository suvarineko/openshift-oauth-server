# redirurl-populator

A tiny controller that keeps an OpenShift **OAuthClient**'s `redirectURIs` in
sync with a required set of URLs on a **remote (central) cluster**.

## Why

In a hub-and-spoke topology many Kubernetes/OpenShift clusters authenticate
through a single **central** cluster that acts as the OAuth identity provider.
For a spoke cluster to log in through the central IdP, the spoke's OAuth
redirect URL(s) must be listed in the `redirectURIs` of the central
`OAuthClient`. Doing that by hand for every new cluster is tedious and
error-prone.

`redirurl-populator` runs alongside each spoke cluster and, on an interval,
ensures the spoke's redirect URLs are present on the central `OAuthClient`,
adding any that are missing. It is **non-destructive**: it never removes or
reorders existing URIs — it only appends the ones that aren't there yet.

```
 ┌────────────────────┐        get / update OAuthClient        ┌────────────────────┐
 │   Spoke cluster    │  ───────────────────────────────────▶  │  Central cluster   │
 │ redirurl-populator │      (bearer token, oauth.openshift.io) │   OAuthClient      │
 └────────────────────┘                                         └────────────────────┘
```

## What it does

On each tick it:

1. `GET`s the `OAuthClient` named `OAUTHCLIENT_NAME` from the remote cluster.
2. Computes which of the desired `REDIRECT_URIS` are missing from its
   `redirectURIs`.
3. If any are missing, appends them and `UPDATE`s the resource. Updates use
   optimistic concurrency (retry-on-conflict), so multiple populators writing to
   the same central `OAuthClient` is safe.

## Configuration (environment variables)

| Variable | Required | Default | Description |
|---|---|---|---|
| `KUBE_API_SERVER` | ✅ | — | API server URL of the central cluster, e.g. `https://api.central.example.com:6443` |
| `KUBE_TOKEN` | ✅¹ | — | Bearer token of the central-cluster ServiceAccount |
| `KUBE_TOKEN_FILE` | ✅¹ | — | Path to a file with the bearer token (takes precedence over `KUBE_TOKEN`; good for mounted/rotated tokens) |
| `OAUTHCLIENT_NAME` | ✅ | — | Name of the `OAuthClient` on the central cluster |
| `REDIRECT_URIS` | ✅ | — | URLs to ensure present. Separators: comma, semicolon, whitespace or newline. Duplicates are removed |
| `OAUTHCLIENT_NAMESPACE` | | _empty_ | Leave empty for a standard **cluster-scoped** `OAuthClient`. Set only if you serve it namespaced |
| `KUBE_INSECURE_SKIP_TLS_VERIFY` | | `true` | Skip verification of the central cluster's serving certificate |
| `KUBE_CA_CERT` | | — | Inline PEM CA bundle (used only when verification is enabled) |
| `KUBE_CA_CERT_FILE` | | — | Path to a PEM CA bundle (used only when verification is enabled) |
| `RECONCILE_INTERVAL` | | `60s` | How often to reconcile (Go duration: `30s`, `5m`, `1h`) |
| `REQUEST_TIMEOUT` | | `30s` | Timeout for a single API call to the central cluster |
| `RUN_ONCE` | | `false` | Reconcile once and exit (exit code reflects success). Useful as a Job/init container |
| `DRY_RUN` | | `false` | Log what would change without writing |
| `HEALTH_ADDR` | | `:8080` | Address for `/healthz` and `/readyz`. Empty disables the server |
| `LOG_LEVEL` | | `info` | `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | | `json` | `json` or `text` |

¹ Provide the token via **either** `KUBE_TOKEN` or `KUBE_TOKEN_FILE`.

> **Security note:** `KUBE_INSECURE_SKIP_TLS_VERIFY` defaults to `true` for
> convenience in lab/self-signed environments. For production, set it to `false`
> and supply the central cluster's CA via `KUBE_CA_CERT`/`KUBE_CA_CERT_FILE`.

### Health endpoints

- `GET /healthz` — liveness; always `200` while the process is up.
- `GET /readyz` — readiness; `200` only after a reconcile has succeeded,
  `503` before the first success or after the most recent reconcile failed.

## Deploy

### 1. On the central cluster — RBAC + token

`OAuthClient` is cluster-scoped, so this needs a `ClusterRole`.

```sh
oc apply -f deploy/rbac-central-cluster.yaml

# Grab the ServiceAccount token for the spoke:
oc -n redirurl-populator get secret redirurl-populator-token \
  -o jsonpath='{.data.token}' | base64 -d
```

### 2. On the spoke cluster — secret, config and deployment

```sh
# Put the token from step 1 into secret.example.yaml, then:
oc apply -f deploy/secret.example.yaml
# Edit deploy/deployment.yaml (KUBE_API_SERVER, OAUTHCLIENT_NAME, REDIRECT_URIS,
# and the image), then:
oc apply -f deploy/deployment.yaml
```

## Build & run

```sh
make build            # -> bin/redirurl-populator (static binary)
make test             # unit tests (no cluster required)
make docker-build     # build the container image

# Run locally against a cluster (dry-run, single pass):
KUBE_API_SERVER=https://api.central.example.com:6443 \
KUBE_TOKEN=... \
OAUTHCLIENT_NAME=spoke-1-oauthclient \
REDIRECT_URIS="https://oauth-openshift.apps.spoke-1.example.com/oauth2callback/central" \
DRY_RUN=true RUN_ONCE=true LOG_FORMAT=text \
  go run .
```

## Design notes

- Uses the Kubernetes **dynamic client** (`unstructured`) so it depends only on
  `client-go`, not on the OpenShift API/client-go modules.
- **Idempotent & non-destructive:** only missing URIs are appended; existing
  URIs and their order are preserved.
- **Concurrency-safe:** writes use `GET` + `UPDATE` with retry-on-conflict, so
  several populators updating the same central `OAuthClient` won't clobber each
  other.
- **Resilient:** in the periodic loop, transient errors (including the target
  `OAuthClient` not existing yet) are logged and retried on the next tick; the
  process keeps running. In `RUN_ONCE` mode an error is fatal (non-zero exit).
