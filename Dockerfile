# Build the manager binary
FROM golang:1.27.3@sha256:2920bb124f1a2e12a1d39f0cab756ae0068441ee1da7655dec7f5e374944095b AS builder
ARG TARGETOS
ARG TARGETARCH
# VERSION-01: build-stamped version (internal/version), passed via --build-arg at release time.
# Defaults keep local `docker build` (no build-args) working, matching internal/version's own
# go-build defaults.
ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the Go source (relies on .dockerignore to filter)
COPY . .

# Build
# the GOARCH has no default value to allow the binary to be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -ldflags="-w -s \
    -X github.com/platformrelay/kollect/internal/version.Version=${VERSION} \
    -X github.com/platformrelay/kollect/internal/version.Commit=${COMMIT} \
    -X github.com/platformrelay/kollect/internal/version.Date=${DATE}" \
    -a -o manager ./cmd

# Runtime image: Debian slim with git + openssh-client for file:// git remotes and git ls-remote probes.
# The only git export engine is go-git (default, ADR-0803), which does not require the git binary; the
# file:// CLI machinery and connection probes do.
FROM debian:bookworm-slim@sha256:99c0b25c46d0214d5a2091092e7908907dcaecd6a962cde0ac700acee222b6f3

RUN apt-get update && \
    apt-get upgrade -y && \
    apt-get install -y --no-install-recommends ca-certificates git openssh-client && \
    rm -rf /var/lib/apt/lists/* && \
    groupadd --gid 65532 nonroot && \
    useradd --uid 65532 --gid 65532 --home-dir /home/nonroot --shell /usr/sbin/nologin --no-create-home nonroot

WORKDIR /
COPY --from=builder /workspace/manager /manager
USER 65532:65532

ENTRYPOINT ["/manager"]
