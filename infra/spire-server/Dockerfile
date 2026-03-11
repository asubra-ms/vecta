# Stage 1: Build the environment
FROM alpine:3.18
RUN addgroup -S spire && adduser -S spire -G spire
RUN apk add --no-cache ca-certificates openssl bash

# Create the directory structure explicitly
RUN mkdir -p /opt/spire/bin /run/spire/data /run/spire/config /var/lib/spire \
    && chown -R spire:spire /opt/spire /run/spire /var/lib/spire

# Directly pull from official images without using intermediate aliases
COPY --from=ghcr.io/spiffe/spire-server:1.8.1 /opt/spire/bin/spire-server /opt/spire/bin/spire-server
COPY --from=ghcr.io/spiffe/spire-agent:1.8.1 /opt/spire/bin/spire-agent /opt/spire/bin/spire-agent

RUN chmod +x /opt/spire/bin/spire-server /opt/spire/bin/spire-agent

USER spire
WORKDIR /opt/spire
ENTRYPOINT ["/opt/spire/bin/spire-server", "run", "-config", "/run/spire/config/server.conf"]

