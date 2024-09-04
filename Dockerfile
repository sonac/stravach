FROM --platform=linux/amd64 golang:1.23-alpine as builder

RUN apk add --no-cache gcc musl-dev sqlite-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

ENV CGO_ENABLED=1 GOOS=linux GOARCH=amd64
RUN go build -ldflags="-w -s" -o stravach ./app/main.go

FROM alpine:latest

RUN apk add --no-cache sqlite-libs
WORKDIR /app
COPY --from=builder /app/stravach /app/stravach
COPY --from=builder /app/templates /app/templates
COPY --from=builder /app/client/dist ./client/dist

EXPOSE 8888
ENTRYPOINT ["./stravach"]

