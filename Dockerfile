FROM golang:1.24-alpine

ENV CGO_ENABLED=0 \
    GO111MODULE=on \
    GOOS=linux
WORKDIR /app

COPY go.mod ./
COPY go.sum ./

RUN go mod download

COPY . .

RUN go build -o main ./cmd/main.go

FROM alpine:3.21

RUN adduser -D appuser

WORKDIR /app

COPY --from=0 /app/main .

USER appuser

EXPOSE 3000

ENTRYPOINT ["./main"]
