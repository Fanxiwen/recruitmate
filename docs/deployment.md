# 测试环境部署指南（阿里云 ECS + 个人免费证书，单域名单证书）

本项目三个服务（外部端 careers、内部端 web、Go API）通过 **一个域名 + 路径分流** 部署，
只消耗一张阿里云个人免费证书（单域名 DV 证书）。拓扑：

```
用户 ──https──▶ Nginx（证书终结）
                 ├─ /        → 外部端静态文件（求职者）
                 ├─ /hr/     → 内部端静态文件（HR，Basic Auth 保护）
                 └─ /api/    → Go API :8080
```

## 一、准备

- 阿里云 ECS（国内节点需域名 ICP 备案；仅为快速演示可用香港/海外节点免备案）
- 免费证书已签发（Nginx 格式：`.pem` + `.key`）
- 域名 A 记录解析到 ECS 公网 IP
- ECS 已安装：Nginx、Go 1.27（编译可本地完成，再上传二进制）

## 二、构建产物

```bash
# 外部端（根路径）
pnpm --filter @recruitmate/careers build

# 内部端（子路径 /hr/，必须与 nginx 配置一致）
VITE_BASE_PATH=/hr pnpm --filter @recruitmate/web build

# Go 后端（CGO 关闭，交叉编译为 Linux 静态二进制）
cd apps/api
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/server ./cmd/server
```

## 三、上传与目录

```bash
scp apps/careers/dist/*        root@服务器:/var/www/recruitmate/careers/   # 递归上传内容
scp apps/web/dist/*            root@服务器:/var/www/recruitmate/hr/        # 递归上传内容
scp apps/api/bin/server        root@服务器:/usr/local/bin/recruitmate-api
scp deploy/nginx.conf.example  root@服务器:/etc/nginx/conf.d/recruit.conf  # 改 server_name 与证书路径
```

> 内部端 dist 必须放到 `hr/` 目录（与 nginx `location /hr/` 的 root 对应）。

## 四、证书与 Nginx

```bash
mkdir -p /etc/nginx/ssl
# 上传证书到 /etc/nginx/ssl/recruit.pem 与 recruit.key
sed -i 's/recruit.example.com/你的域名/g' /etc/nginx/conf.d/recruit.conf
nginx -t && systemctl reload nginx
```

内部端 Basic Auth（演示用，非正式鉴权）：

```bash
htpasswd -c /etc/nginx/.hr-htpasswd demo   # 输入一个演示密码
```

## 五、启动 Go API

用 systemd 托管：

```ini
# /etc/systemd/system/recruitmate-api.service
[Unit]
Description=Recruitmate API
After=network.target postgresql.service redis.service

[Service]
WorkingDirectory=/opt/recruitmate
EnvironmentFile=/opt/recruitmate/.env
ExecStart=/usr/local/bin/recruitmate-api
Restart=always

[Install]
WantedBy=multi-user.target
```

`/opt/recruitmate/.env` 关键项：

```env
APP_ENV=prod
HTTP_ADDR=:8080
DATABASE_URL=postgres://recruitmate:你的密码@127.0.0.1:5432/recruitmate?sslmode=disable
REDIS_ADDR=127.0.0.1:6379
JWT_SECRET=改成一段强随机串
S3_ENDPOINT=127.0.0.1:9000
# 同域部署无需 CORS；保留默认即可
```

```bash
systemctl daemon-reload && systemctl enable --now recruitmate-api
```

首次运行会自动执行数据库迁移（goose Up）。种子数据按需：

```bash
cd /opt/recruitmate && /usr/local/bin/recruitmate-api --seed 2>/dev/null || go run ./cmd/seed
```

## 六、注意事项

- **免费证书有效期 3 个月**：在阿里云控制台开启自动续期（DNS 验证），或到期手动续；
- **备案**：ECS 为国内节点时，80/443 对外需域名 ICP 备案；仅为面试演示可用香港/海外节点规避；
- **内部端与外部端同域**：localStorage key 已隔离（`recruitmate.careers.token` vs `recruitmate-auth`），互不影响；
- **安全**：`JWT_SECRET` 必须改强随机值；内部端建议至少保留 Nginx Basic Auth，或后续加企业 SSO。
