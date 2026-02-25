FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/processing-service ./cmd/processing-service

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata ffmpeg

WORKDIR /app

COPY --from=builder /bin/processing-service /usr/local/bin/processing-service

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/processing-service"]
