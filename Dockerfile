ARG TARGETPLATFORM

FROM alpine:latest AS certs
ARG TARGETPLATFORM
RUN apk update && apk add ca-certificates

FROM cgr.dev/chainguard/busybox:latest
ARG TARGETPLATFORM
COPY --from=certs /etc/ssl/certs /etc/ssl/certs

COPY $TARGETPLATFORM/prometheus-mcp-server /usr/bin/prometheus-mcp-server

USER nobody
ENTRYPOINT ["/usr/bin/prometheus-mcp-server"]
