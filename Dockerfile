FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /plexmatch-webhook .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /plexmatch-webhook /usr/local/bin/plexmatch-webhook

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/plexmatch-webhook"]
