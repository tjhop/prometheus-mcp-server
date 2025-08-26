FROM --platform=$BUILDPLATFORM golang:1.25-bookworm AS build
WORKDIR /src
ENV CGO_ENABLED=0
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETOS TARGETARCH
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags='-s -w' \
    -o /out/prometheus-mcp-server ./cmd/prometheus-mcp-server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/prometheus-mcp-server /usr/bin/prometheus-mcp-server
# distroless:static doesn't include CAs; copy them from builder
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
USER nonroot:nonroot
ENTRYPOINT ["/usr/bin/prometheus-mcp-server"]
