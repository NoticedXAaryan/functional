FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git
WORKDIR /app
COPY services/worker/go.mod services/worker/go.sum ./
RUN go mod download
COPY services/worker/ .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/worker .

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /bin/worker /app/worker
CMD ["/app/worker"]
