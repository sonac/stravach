FROM golang:1.22 as builder

ENV GO11MODULE=on

WORKDIR /app
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 go build -a -installsuffix cgo -o stravach ./app/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/stravach .

EXPOSE 8888

CMD ["./stravach"]
