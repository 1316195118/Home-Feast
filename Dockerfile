# syntax=docker/dockerfile:1
# 三阶段构建：前端静态产物 -> 嵌入 Go 单文件 -> 极简运行镜像。
# 运行镜像里没有 Node.js、没有数据库，只有一个二进制和时区证书。

FROM node:20-alpine AS frontend
WORKDIR /build/web
# 国内网络下固定使用可达的 npm 镜像，只影响构建容器，不改变源码配置。
ENV NPM_CONFIG_REGISTRY=https://registry.npmmirror.com
COPY web/package.json ./
RUN npm install --no-audit --no-fund
COPY web/ ./
RUN npm run build

FROM golang:1.24-alpine AS backend
WORKDIR /build
COPY go.mod embed.go ./
COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN mkdir -p webapp/dist
COPY --from=frontend /build/web/dist/ ./webapp/dist/
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /out/menu ./cmd/menu

FROM alpine:3.21
RUN apk add --no-cache tzdata ca-certificates wget
ENV TZ=Asia/Shanghai
COPY --from=backend /out/menu /usr/local/bin/menu
RUN mkdir -p /data/uploads
VOLUME ["/data"]
EXPOSE 8090
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8090/api/ping >/dev/null || exit 1
ENTRYPOINT ["/usr/local/bin/menu"]
CMD ["-addr", "0.0.0.0", "-port", "8090", "-data", "/data"]
