FROM alpine:latest AS certs
RUN apk update && apk add ca-certificates

FROM cgr.dev/chainguard/busybox:latest
COPY --from=certs /etc/ssl/certs /etc/ssl/certs

COPY prometheus-mcp-server /usr/bin/prometheus-mcp-server

USER nobody
ENTRYPOINT ["/usr/bin/prometheus-mcp-server"]
