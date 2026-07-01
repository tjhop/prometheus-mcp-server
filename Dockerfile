ARG ARCH="amd64"
ARG OS="linux"
FROM quay.io/prometheus/busybox-${OS}-${ARCH}:latest
LABEL maintainer="The Prometheus Authors <prometheus-developers@googlegroups.com>"

ARG ARCH="amd64"
ARG OS="linux"
COPY .build/${OS}-${ARCH}/prometheus-mcp /bin/prometheus-mcp

USER nobody
EXPOSE     8080
ENTRYPOINT [ "/bin/prometheus-mcp" ]
