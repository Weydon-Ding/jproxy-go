FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/jproxy ./cmd/jproxy

FROM alpine:3.20
RUN adduser -D -H -u 1000 app
USER app
WORKDIR /app
COPY --from=builder /out/jproxy /app/jproxy
EXPOSE 8117
ENTRYPOINT ["/app/jproxy"]
