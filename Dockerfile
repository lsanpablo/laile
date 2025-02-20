# Build stage: Compile both binaries.
FROM golang:1.20-alpine AS builder

WORKDIR /app

# Cache dependencies.
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code.
COPY . .

# Build the webserver binary.
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o webserver ./cmd/webserver/main.go

# Build the worker binary.
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o worker ./cmd/worker/main.go

# Final stage: Create a lightweight image.
FROM alpine:latest

# Install certificates if your app makes HTTPS calls.
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy the binaries from the builder stage.
COPY --from=builder /app/webserver .
COPY --from=builder /app/worker .

# Copy the production configuration file into /etc.
COPY webhook_config.toml /etc/webhook_config.toml

# Copy the entrypoint script.
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Set environment variable to use the configuration from /etc in production.
ENV CONFIG_PATH=/etc/webhook_config.toml

# Set the entrypoint.
ENTRYPOINT ["/entrypoint.sh"]
