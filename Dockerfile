FROM golang:1.22 as builder

ENV GO11MODULE=on

WORKDIR /app
COPY . .
RUN go build -o stravach ./app/main.go

EXPOSE 8888

ENTRYPOINT ["./stravach"]