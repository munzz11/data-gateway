# Use Go 1.21 as base image
FROM golang:1.21 AS builder

WORKDIR /app

# Install SQLite dependencies
RUN apt-get update && apt-get install -y \
    gcc \
    libsqlite3-dev \
    && rm -rf /var/lib/apt/lists/*

# Copy the go.mod and go.sum files to the /app directory
COPY go.mod go.sum ./

# Install dependencies
RUN go mod download

# Copy the entire source code into the container
COPY . .

# Create data directory in builder stage
RUN mkdir -p /app/data && chmod 777 /app/data

# Build the application with CGO enabled
RUN CGO_ENABLED=1 GOOS=linux go build -o main .

# Use Ubuntu 22.04 as the final base image
FROM ubuntu:22.04

WORKDIR /app

# Install SQLite runtime dependencies
RUN apt-get update && apt-get install -y \
    libsqlite3-0 \
    && rm -rf /var/lib/apt/lists/*

# Copy the binary and data directory from builder
COPY --from=builder /app/main .
COPY --from=builder /app/data ./data

# Create data directory in final stage (in case the copy fails)
RUN mkdir -p /app/data && chmod 777 /app/data

# Document the port that needs to be published
EXPOSE 8080

# Start the application
CMD ["./main"] 