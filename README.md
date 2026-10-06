# TinyURL - 自建短链服务

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Gin](https://img.shields.io/badge/Gin-v1.9+-FF6F61?logo=go&logoColor=white)](https://github.com/gin-gonic/gin)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

一个用 **Golang + Gin** 开发的**高性能、自托管短链服务**，支持用户认证、自定义短码、链接过期、点击统计、IP 限流等生产级功能。

线上地址：https://tinyurl.mahiro.cloud/legacy/

## 功能特性

- 🚀 **核心短链**：输入长链接 → 生成短链 → 302 重定向
- 🔐 **用户系统**：注册/登录（JWT 认证） + 个人短链列表
- ✂️ **自定义短码**：用户可指定短码（如 /myblog），自动检查冲突
- ⏰ **链接过期**：支持设置过期时间（默认 30 天）
- 📊 **访问统计**：实时点击数（Redis 存储）
- 🛡️ **安全限流**：IP + 用户维度限流，防刷
- ⚡ **高性能**：Redis 缓存 + SQLite 持久化
- 🌐 **HTTPS 部署**：Nginx 反向代理 + Let's Encrypt 免费证书

## 技术栈

- **后端**：Go 1.21+ + Gin
- **数据库**：SQLite（持久化） + Redis（缓存 & 计数）
- **认证**：JWT + bcrypt 密码哈希
- **限流**：ulule/limiter
- **部署**：可执行文件 + Nginx 反向代理 + HTTPS

## 快速开始

### 1. 本地运行

```bash
# 克隆仓库
git clone https://github.com/GenJi77JYXC/tinyURL.git
cd tinyURL

# 安装依赖
go mod tidy

# 复制示例配置
cp .env.example .env
# 编辑 .env（修改 BASE_URL、JWT_SECRET 等）

# 启动 Redis（Docker 方式）
docker run -d -p 6379:6379 --name redis-tinyurl redis:7-alpine

# 运行服务（默认端口 8080）
go run ./cmd
```

### 2. 生成短链（需登录）

先注册/登录获取 token：

```bash
# 注册
curl -X POST http://localhost:8080/api/register \
  -H "Content-Type: application/json" \
  -d '{"username":"test","password":"123456"}'

# 登录
curl -X POST http://localhost:8080/api/login \
  -H "Content-Type: application/json" \
  -d '{"username":"test","password":"123456"}'
```

创建短链（带 token）：

```bash
curl -X POST http://localhost:8080/api/shorten \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer 你的token" \
  -d '{"url": "https://www.example.com", "custom_code": "myblog", "expire_days": 7}'
```

### 3. 部署到服务器（Linux）

```bash
# 交叉编译
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o tinyurl-linux ./cmd

# 上传到服务器
scp tinyurl-linux user@你的服务器IP:/home/user/

# 服务器上操作
chmod +x tinyurl-linux
sudo mv tinyurl-linux /usr/local/bin/tinyurl

# 创建 systemd 服务
sudo tee /etc/systemd/system/tinyurl.service <<EOF
[Unit]
Description=TinyURL Service
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/home/user
EnvironmentFile=/home/user/.env
ExecStart=/usr/local/bin/tinyurl
Restart=always

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl start tinyurl
sudo systemctl enable tinyurl
```

## 项目结构

```text
tinyURL/
├── cmd/
│   └── main.go              # 入口
├── internal/
│   ├── api/                 # Handler & Router & Middleware
│   │   ├── handler.go
│   │   ├── router.go
│   │   └── middleware.go
│   ├── model/               # 数据模型（User, Link）
│   ├── repository/          # SQLite & Redis 存储层
│   ├── service/             # 业务逻辑（Shortener & Auth）
│   └── config/              # 配置（JWT 等）
├── pkg/util/                # Base62 编码等工具
├── .env                     # 环境变量
└── README.md
```

## API 接口文档

| 方法 | 路径 | 说明 | 鉴权 |
|------|------|------|------|
| POST | `/api/register` | 用户注册 | 否 |
| POST | `/api/login` | 登录返回 JWT | 否 |
| POST | `/api/shorten` | 创建短链 | 是 |
| GET | `/api/my-links` | 我的短链列表 | 是 |
| GET | `/:short` | 短链 302 重定向 | 否 |
| GET | `/api/stats/:short` | 查看点击统计 | 否 |

## 线上演示

- **旧前端**（带用户系统，本项目）：https://tinyurl.mahiro.cloud/legacy/
- **新前端**（无登录，简化版）：https://tinyurl.mahiro.cloud/
- 短链示例：https://tinyurl.mahiro.cloud/6 → 跳转到 https://www.baidu.com

## 贡献 & License

欢迎提交 Issue/PR！

本项目采用 MIT License

© 2026 GenJi (@GenJi77JYXC)
