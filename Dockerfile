# Stage 1: Build the Go application with CGO enabled
FROM --platform=linux/amd64 golang:1.23-alpine as builder

# Install required dependencies for CGO
RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /app

# Copy the Go modules manifest
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the rest of the source code
COPY . .

# Enable CGO and build the application binary
ENV CGO_ENABLED=1 GOOS=linux GOARCH=amd64
RUN go build -ldflags="-w -s" -o stravach ./app/main.go

# Stage 2: Create the final minimal image
FROM alpine:latest

# Install SQLite dependencies
RUN apk add --no-cache sqlite-libs

# Set the working directory
WORKDIR /app

# Copy the binary and templates from the builder stage
COPY --from=builder /app/stravach /app/stravach
COPY --from=builder /app/templates /app/templates

# Expose the required port
EXPOSE 8888

# Set the entrypoint to run the binary
ENTRYPOINT ["./stravach"]

