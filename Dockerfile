# Use the official Golang image as a base image
FROM golang:1.23-alpine AS builder

# Set the current working directory inside the container
WORKDIR /app

# Set Go toolchain to local to prevent version conflicts
ENV GOTOOLCHAIN=local

# Copy go.mod and go.sum files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code into the container
COPY . .

# Build the Go application
RUN go build -o /go-auth-api ./cmd/api

# Use a minimal image for the final stage
FROM alpine:latest

# Install ca-certificates for HTTPS connections
RUN apk --no-cache add ca-certificates

# Set the current working directory inside the container
WORKDIR /root/

# Copy the built binary from the builder stage
COPY --from=builder /go-auth-api .

# Expose the port the application runs on
EXPOSE 8080

# Run the application
CMD ["./go-auth-api"]