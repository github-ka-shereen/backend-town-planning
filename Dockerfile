# syntax=docker/dockerfile:1

# Create a stage for building the application
ARG GO_VERSION=1.23.2
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS build

# Set the working directory for the build stage
WORKDIR /src

# Download dependencies
RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,source=go.sum,target=go.sum \
    --mount=type=bind,source=go.mod,target=go.mod \
    go mod download -x

# Build the application
RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,target=. \
    CGO_ENABLED=0 GOARCH=$TARGETARCH go build -o /bin/server ./cmd

# Create a new stage for running the application
FROM alpine:latest AS final

# Install runtime dependencies
RUN --mount=type=cache,target=/var/cache/apk \
    apk --update add \
        ca-certificates \
        tzdata \
        && \
        update-ca-certificates

# Create a non-privileged user to run the app
ARG UID=10001
RUN adduser \
    --disabled-password \
    --gecos "" \
    --home "/nonexistent" \
    --shell "/sbin/nologin" \
    --no-create-home \
    --uid "${UID}" \
    appuser
USER appuser

# Set the working directory where the server will run
WORKDIR /app

# Copy the executable from the build stage
COPY --from=build /bin/server /bin/

# Copy the .env file into the container
COPY .env /app/.env

# Ensure the public/files directory exists and is writable
RUN mkdir -p /app/public/files && \
    chown -R appuser:appuser /app/public

# Expose the application port
EXPOSE 8080

# Start the application when the container runs
ENTRYPOINT [ "/bin/server" ]