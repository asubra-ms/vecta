# Stage 1: Grab the official binaries
FROM ghcr.io/spiffe/spire-server:1.8.1 AS source

# Stage 2: Build the clean production image
FROM alpine:3.18

# Create the spire user/group
RUN addgroup -S spire && adduser -S spire -G spire

# Install necessary tools
RUN apk add --no-cache ca-certificates openssl bash

# Create directories
RUN mkdir -p /opt/spire/bin /run/spire/data /run/spire/config /var/lib/spire \
    && chown -R spire:spire /opt/spire /run/spire /var/lib/spire

# Copy the server binary
COPY --from=source /opt/spire/bin/spire-server /opt/spire/bin/spire-server

USER spire
WORKDIR /opt/spire

ENTRYPOINT ["/opt/spire/bin/spire-server", "run", "-config", "/run/spire/config/server.conf"]
