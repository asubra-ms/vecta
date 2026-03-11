# Stage 1: Grab the official binaries
FROM ghcr.io/spiffe/spire-server:1.8.1 AS source

# Stage 2: Build the clean production image
FROM alpine:3.18

# Create the spire user/group
RUN addgroup -S spire && adduser -S spire -G spire

# Install necessary tools
RUN apk add --no-cache ca-certificates openssl bash

# Create directories using the new sovereign standard
RUN mkdir -p /usr/local/vecta/bin /run/spire/data /run/spire/config /var/lib/spire \
    && chown -R spire:spire /usr/local/vecta /run/spire /var/lib/spire

# Copy BOTH server and agent binaries to the sovereign path
COPY --from=source /opt/spire/bin/spire-server /usr/local/vecta/bin/spire-server
COPY --from=source /opt/spire/bin/spire-agent /usr/local/vecta/bin/spire-agent

USER spire
WORKDIR /usr/local/vecta

# Entrypoint updated to sovereign binary path
ENTRYPOINT ["/usr/local/vecta/bin/spire-server", "run", "-config", "/run/spire/config/server.conf"]

