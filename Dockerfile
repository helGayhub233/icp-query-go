# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /icpquery .

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app

COPY --from=builder /icpquery /app/icpquery
COPY config.yml /app/config.yml

EXPOSE 8080

ENTRYPOINT ["/app/icpquery"]
CMD ["serve"]
