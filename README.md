# TinyURL - 自建短链服务

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Gin](https://img.shields.io/badge/Gin-v1.9+-FF6F61?logo=go&logoColor=white)](https://github.com/gin-gonic/gin)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Deployment](https://img.shields.io/badge/Deployed-genji.xin-blue)](https://genji.xin)

一个用 **Golang + Gin** 开发的**高性能、自托管短链服务**，支持用户认证、自定义短码、链接过期、点击统计、IP 限流等生产级功能。

项目已部署到公网：https://genji.xin（后端）

## 功能特性

- 🚀 **核心短链**：输入长链接 → 生成短链 → 永久/临时重定向
- 🔐 **用户系统**：注册/登录（JWT 认证） + 个人短链列表
- ✂️ **自定义短码**：用户可指定短码（如 /myblog），自动检查冲突
- ⏰ **链接过期**：支持设置过期时间（默认 30 天）
- 📊 **访问统计**：实时总点击 + 日点击（Redis 存储）
- 🛡️ **安全限流**：IP + 用户维度限流，防刷
- ⚡ **高性能**：Redis 缓存 + SQLite 持久化，2核2G 服务器轻松支持
- 🌐 **HTTPS 部署**：Nginx 反向代理 + Let's Encrypt 免费证书
- 📱 **前端友好**：支持 SPA 前端（如 Vue/React）集成

## 技术栈

- **后端**：Go 1.21+ + Gin（Web 框架）
- **数据库**：SQLite（持久化） + Redis（缓存 & 计数）
- **认证**：JWT + bcrypt 密码哈希
- **限流**：ulule/limiter
- **部署**：systemd 服务 + Nginx 反向代理 + HTTPS
- **配置**：viper + .env

## 快速开始

### 1. 本地运行

```bash
# 克隆仓库
git clone https://github.com/GenJi77JYXC/tinyurl.git
cd tinyurl

# 安装依赖
go mod tidy

# 复制示例配置
cp .env.example .env
# 编辑 .env（修改 BASE_URL、JWT_SECRET 等）

# 启动 Redis（Docker 方式推荐）
docker run -d -p 6379:6379 --name redis-tinyurl redis:latest

# 运行服务
go run cmd/main.go