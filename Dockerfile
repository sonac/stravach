# Stage 1: Build the Go application
FROM golang:1.22 as builder

WORKDIR /app
COPY . .
RUN go build -o stravach ./app/main.go

EXPOSE 8888

ENTRYPOINT ["./stravach"]
