FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git
WORKDIR /app
COPY services/api/go.mod services/api/go.sum ./
RUN go mod download
COPY services/api/ .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/server ./cmd/server/

FROM alpine:3.19
RUN apk add --no-cache wget ca-certificates
WORKDIR /app
COPY --from=builder /bin/server /app/server
COPY db/migrations /app/db/migrations
EXPOSE 8080
CMD ["/app/server"]
