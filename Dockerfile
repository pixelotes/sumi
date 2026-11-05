FROM golang:1.27-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o sumi ./cmd/sumi

FROM alpine:3.24

RUN apk add --no-cache tzdata

WORKDIR /app
COPY --from=builder /build/sumi .
COPY --from=builder /build/web ./web

EXPOSE 8085

ENTRYPOINT ["./sumi", "-config", "/app/config/config.yml"]
