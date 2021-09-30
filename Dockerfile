# Build the manager binary
FROM golang:1.16-alpine3.14 AS builder

WORKDIR /workspace

RUN go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.4.1

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY cmd/ cmd/
COPY internal/ internal/
COPY hack/ hack/

ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64

RUN controller-gen object:headerFile="./hack/boilerplate.go.txt" paths="./..."

# Build
RUN go build -a -o manager cmd/cache/main.go
RUN go build -a -o registry-proxy cmd/proxy/main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager /usr/local/bin/
COPY --from=builder /workspace/registry-proxy /usr/local/bin/
USER 65532:65532

ENTRYPOINT ["manager"]