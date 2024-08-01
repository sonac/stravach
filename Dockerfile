# Stage 1: Build the Go application
FROM golang:1.22 as builder

ENV GO111MODULE=on

WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o stravach ./app/main.go

# Stage 2: Create a minimal image with the built Go binary
FROM debian:bullseye-slim
RUN apt-get update && apt-get install -y ca-certificates

WORKDIR /root/
COPY --from=builder /app/stravach .

EXPOSE 8888

CMD ["./stravach"]
