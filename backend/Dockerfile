FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
    go build -o app main.go

# ------ runtime ------
FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/app .
COPY --from=builder /app/static ./static

RUN apk --no-cache add ca-certificates

EXPOSE 8080

USER nobody

CMD ["./app"]