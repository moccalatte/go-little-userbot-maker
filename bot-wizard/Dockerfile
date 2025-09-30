# syntax=docker/dockerfile:1.7

FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/botwizard ./cmd/botwizard

FROM alpine:3.20
RUN addgroup -S app && adduser -S app -G app
WORKDIR /home/app
COPY --from=builder /out/botwizard /usr/local/bin/botwizard
RUN mkdir -p /home/app/storage/logs && chown -R app:app /home/app /usr/local/bin/botwizard
USER app
ENV APP_ENV=production
ENTRYPOINT ["botwizard"]
