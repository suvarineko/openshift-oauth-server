# syntax=docker/dockerfile:1

ARG GO_VERSION=1.23
FROM golang:${GO_VERSION} AS build
WORKDIR /src

# Cache module downloads.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X main.version=${VERSION}" \
    -o /out/redirurl-populator .

# Minimal, non-root runtime image. Includes CA certificates for the case where
# TLS verification against the remote cluster is enabled.
FROM gcr.io/distroless/static:nonroot
LABEL org.opencontainers.image.title="redirurl-populator" \
      org.opencontainers.image.description="Ensures an OpenShift OAuthClient's redirectURIs contain a required set of URLs" \
      org.opencontainers.image.source="https://github.com/suvarineko/redirurl-populator"

COPY --from=build /out/redirurl-populator /usr/local/bin/redirurl-populator
USER 65532:65532
ENTRYPOINT ["/usr/local/bin/redirurl-populator"]
