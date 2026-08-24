# 多阶段构建：编译 → alpine 运行（静态二进制 + seed 工具）
# 说明：同时产出 /server（API 服务）与 /seed（种子数据工具），
# 运行 seed 用：docker run --rm --entrypoint /seed <image>
FROM golang:1.27-alpine AS build
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /src

# 先拷贝依赖清单，利用镜像层缓存
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /server ./cmd/server \
 && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /seed ./cmd/seed

FROM alpine:3.20
RUN adduser -D -u 10001 app
COPY --from=build /server /server
COPY --from=build /seed /seed
COPY migrations /migrations

EXPOSE 8080
ENTRYPOINT ["/server"]
