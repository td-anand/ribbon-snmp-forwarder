# ---- Build stage --------------------------------------------------------
FROM golang:1.24-alpine AS builder

# Install ca-certificates for TLS and build tools
RUN apk add --no-cache ca-certificates git make

WORKDIR /build

# Cache dependency downloads before copying full source
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build a statically-linked binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" \
    -o /bin/ribbon-snmp-forwarder ./cmd/server

# ---- Runtime stage ------------------------------------------------------
FROM scratch

LABEL org.opencontainers.image.title="ribbon-snmp-forwarder" \
      org.opencontainers.image.description="SNMP trap receiver and poller that forwards Ribbon SBC telemetry to Kafka" \
      org.opencontainers.image.source="https://github.com/td-anand/ribbon-snmp-forwarder"

# Copy TLS certificates so Kafka TLS connections work
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy the statically-linked binary
COPY --from=builder /bin/ribbon-snmp-forwarder /ribbon-snmp-forwarder

# SNMP trap listener (UDP 162).  Note: binding to port <1024 requires
# the NET_BIND_SERVICE capability.  Add it to your Podman run command:
#   --cap-add=NET_BIND_SERVICE
# or use a higher port (e.g. 1162) in config and redirect with iptables.
EXPOSE 162/udp

ENTRYPOINT ["/ribbon-snmp-forwarder"]
CMD ["-config", "/etc/ribbon-snmp-forwarder/config.yaml"]
