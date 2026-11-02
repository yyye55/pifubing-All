# ---- 构建阶段 ----
FROM golang:1.23-alpine AS builder

WORKDIR /app

# 先复制依赖文件，利用 Docker 层缓存
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# 纯 Go 的 SQLite 驱动不需要 CGO，静态编译方便部署
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o server .

# ---- 运行阶段 ----
FROM alpine:3.20

WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 1000 appuser

COPY --from=builder /app/server .
RUN mkdir -p /app/uploads /data && chown -R appuser:appuser /app /data

USER appuser
EXPOSE 9094

CMD ["./server"]
