# 多阶段构建：编译 → distroless 运行（静态二进制，无 shell，最小攻击面）
FROM golang:1.27-alpine AS builder

WORKDIR /src

# 先拷贝依赖清单，利用镜像层缓存
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/server /server
COPY migrations /migrations

EXPOSE 8080
ENTRYPOINT ["/server"]
