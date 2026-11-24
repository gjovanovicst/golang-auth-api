# syntax=docker/dockerfile:1

# Build stage
FROM golang:1.24-alpine AS builder

# Set the current working directory inside the container
WORKDIR /app

# Set Go toolchain to local to prevent version conflicts
ENV GOTOOLCHAIN=local
# Static binary: no libc dependency, cross-compiles cleanly for linux/amd64 and linux/arm64
ENV CGO_ENABLED=0

# Copy go.mod and go.sum files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code into the container
COPY . .

# Version and commit are supplied by the release workflow
ARG VERSION=dev
ARG COMMIT=none

# Build the Go application
RUN go build -trimpath \
      -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
      -o /go-auth-api ./cmd/api

# Minimal runtime image, running as an unprivileged user
FROM alpine:latest

# Install ca-certificates for HTTPS connections and create a non-root user
RUN apk --no-cache add ca-certificates \
 && addgroup -S app \
 && adduser -S -G app app

WORKDIR /home/app

# Copy the built binary from the builder stage
COPY --from=builder /go-auth-api .

# Production defaults: Gin release mode, no debug route dump
ENV GIN_MODE=release

USER app

# Expose the port the application runs on
EXPOSE 8080

# Run the application
CMD ["./go-auth-api"]
