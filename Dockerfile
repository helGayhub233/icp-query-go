# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /icpcli .

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app

COPY --from=builder /icpcli /app/icpcli
COPY config.yml /app/config.yml

# Default to MCP server mode for AI agent integration
# Override with: docker run ... icpcli query baidu.com
ENTRYPOINT ["/app/icpcli"]
CMD ["mcp"]
