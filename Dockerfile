# Stage 1: Build the Go application
FROM golang:1.22 as builder

ENV GO111MODULE=on

WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o stravach ./app/main.go

# Stage 2: Create a minimal image with the built Go binary
FROM alpine:latest
RUN apk --no-cache add ca-certificates libc6-compat

WORKDIR /root/
COPY --from=builder /app/stravach .

EXPOSE 8888

CMD ["./stravach"]
