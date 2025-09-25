# Stage 1: Build the Go binary
FROM golang:1.23 AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy the Go modules and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the application source code
COPY . .

# Build the Go application
RUN CGO_ENABLED=0 GOOS=linux go build -o server .

# Stage 2: Create a small image with only the Go binary
FROM alpine:latest

# Install certificates for HTTPS, if needed
RUN apk --no-cache add ca-certificates

# Set the working directory inside the container
WORKDIR /root/

# Copy the compiled binary from the builder stage
COPY --from=builder /app/server .

# Expose the port on which the application runs
EXPOSE 5000

# Run the Go application
CMD ["./server"]
