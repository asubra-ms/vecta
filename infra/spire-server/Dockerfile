FROM ghcr.io/spiffe/spire-server:1.8.1 AS source
FROM alpine:3.18

RUN addgroup -S spire && adduser -S spire -G spire
RUN apk add --no-cache ca-certificates openssl bash

# Create the sovereign bin directory
RUN mkdir -p /usr/local/vecta/bin /run/spire/data /run/spire/config /var/lib/spire \
    && chown -R spire:spire /usr/local/vecta /run/spire /var/lib/spire

# Copy binaries to the exact bin path
COPY --from=source /opt/spire/bin/spire-server /usr/local/vecta/bin/spire-server
COPY --from=source /opt/spire/bin/spire-agent /usr/local/vecta/bin/spire-agent

RUN chmod +x /usr/local/vecta/bin/spire-server /usr/local/vecta/bin/spire-agent

USER spire
WORKDIR /usr/local/vecta
ENTRYPOINT ["/usr/local/vecta/bin/spire-server", "run", "-config", "/run/spire/config/server.conf"]

