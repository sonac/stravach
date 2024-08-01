# Stage 1: Build the Go application
FROM golang:1.22 as builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o stravach ./app/main.go
ENTRYPOINT ["./stravach"]
