FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/server ./cmd/server/

FROM alpine:3.19
RUN apk add --no-cache wget ca-certificates
WORKDIR /app
COPY --from=builder /bin/server /app/server
EXPOSE 8080
CMD ["/app/server"]
