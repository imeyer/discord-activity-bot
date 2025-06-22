# Runtime stage - use distroless for minimal attack surface
FROM gcr.io/distroless/static-debian12:nonroot

# Copy the pre-built binary (built by Bazel in CI)
COPY discord-activity-bot /usr/local/bin/discord-activity-bot

# Create a non-root user
USER nonroot:nonroot

# Expose any ports if needed (Discord bots typically don't need exposed ports)
# EXPOSE 8080

# Add labels for better container metadata
ARG VERSION=unknown
ARG BUILD_DATE=unknown
ARG GIT_COMMIT=unknown

LABEL org.opencontainers.image.title="Discord Activity Bot"
LABEL org.opencontainers.image.description="A Discord bot for tracking server activity"
LABEL org.opencontainers.image.version="${VERSION}"
LABEL org.opencontainers.image.created="${BUILD_DATE}"
LABEL org.opencontainers.image.revision="${GIT_COMMIT}"
LABEL org.opencontainers.image.source="https://github.com/imeyer/discord-activity-bot"
LABEL org.opencontainers.image.licenses="MIT"

# Health check (optional - remove if not needed)
# HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
#   CMD ["/usr/local/bin/discord-activity-bot", "--health-check"]

ENTRYPOINT ["/usr/local/bin/discord-activity-bot"]