# syntax=docker/dockerfile:1.7

FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/userbot ./cmd/userbot

FROM alpine:3.20
RUN addgroup -S app && adduser -S app -G app
WORKDIR /home/app
COPY --from=builder /out/userbot /usr/local/bin/userbot
RUN mkdir -p /home/app/storage/logs && chown -R app:app /home/app /usr/local/bin/userbot
USER app
ENV APP_ENV=production
ENTRYPOINT ["userbot"]
