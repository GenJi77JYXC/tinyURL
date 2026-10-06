# TinyURL — Go 高并发短链服务

[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Gin](https://img.shields.io/badge/Gin-v1.11-FF6F61)](https://github.com/gin-gonic/gin)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-336791?logo=postgresql&logoColor=white)](https://www.postgresql.org/)
[![Redis](https://img.shields.io/badge/Redis-7-DC382D?logo=redis&logoColor=white)](https://redis.io/)
[![Prometheus](https://img.shields.io/badge/Prometheus-metrics-E6522C?logo=prometheus&logoColor=white)](https://prometheus.io/)

一个用 Go 实现的高并发短链服务：**自增 ID + Base62 定长短码**、**Redis cache-aside**、
**按 IP 令牌桶限流**、**SSRF 防护**、**Prometheus 指标**，一条 `docker compose up`
拉起 PostgreSQL 16 / Redis 7 / Prometheus / Grafana 全套环境。

## 架构

```text
                         ┌──────────────────────────────────────────────┐
                         │                  :8080 (Gin)                  │
  Client ──POST /shorten►│  RateLimit(x/time/rate, per IP) → Handler     │
                         │              ↓                                 │
                         │        Core Service                            │
                         │   ValidateLongURL(SSRF) → nextval → Base62(6)  │
                         │              ↓                                 │
  Client ──GET /s/{code}►│  Resolve: Redis → miss → PostgreSQL → 回写缓存 │
        302 Found ◄──────┤                                              │
                         │  /metrics (Prometheus)  /healthz              │
                         └───────┬───────────────────────┬────────────────┘
                                 │                       │
                      ┌────────────────────┐   ┌────────────────────┐
                      │  Redis 7           │   │  PostgreSQL 16     │
                      │  short:{code} URL  │   │  links: id BIGSERIAL
                      │  "" 空值防穿透      │   │  short_code CHAR(6)│
                      │  TTL 1h / 负缓存1m │   │  UNIQUE            │
                      └────────────────────┘   └────────────────────┘
                                 ▲
                  Prometheus 9090 │ scrape 5s
                                 │
                         Grafana 3000 (预置看板)
```

跳转一律返回 **302 Found**（而非 301），保证每次点击都经过服务，便于统计与灰度切换。

## 在线体验

已部署到生产环境：`https://tinyurl.mahiro.cloud`（阿里云 2C2G + Docker Compose + Nginx 反代 + HTTPS）

```bash
# 1. 健康检查
curl https://tinyurl.mahiro.cloud/healthz
# {"status":"ok"}

# 2. 创建一条短链
curl -X POST https://tinyurl.mahiro.cloud/shorten \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://example.com"}'
# 201 {"short_code":"100001","short_url":"https://tinyurl.mahiro.cloud/s/100001"}

# 3. 验证跳转（302 Found → Location 指向原始 URL）
curl -s -o /dev/null -w "%{http_code} %{redirect_url}\n" \
  https://tinyurl.mahiro.cloud/s/100001
# 302 https://example.com/
```

直接在浏览器打开 `https://tinyurl.mahiro.cloud/s/100001` 也能看到 302 跳转效果。

监控看板（Grafana）：`https://tinyurl.mahiro.cloud/grafana/`（访客暂不提供账号，本地复现时 admin/admin 即可看到同款看板）

> 注意：生成接口有 10 QPS/IP 的令牌桶限流，频繁调用会收到 429；这是刻意设计的防护行为。

## 技术栈

| 关注点 | 选型 |
|---|---|
| 语言 / Web | Go 1.25、Gin v1.11 |
| 数据库 | PostgreSQL 16（pgx v5 + database/sql，启动时 go:embed 自动迁移） |
| 缓存 | Redis 7（go-redis v9，连接池埋点；故障自动降级直查 DB） |
| 短码 | 自增 ID 偏移 `62^5` 后 Base62，**恒定 6 位**，可用容量约 558 亿 |
| 限流 | `golang.org/x/time/rate` 令牌桶，默认 10 QPS/IP、突发 5 |
| 安全 | 仅允许 http/https；DNS 解析后拦截 loopback/私网/链路本地/CGNAT（防 SSRF） |
| 监控 | Prometheus client_golang：Histogram / Counter / Gauge |
| 部署 | 多阶段 Dockerfile（静态二进制 + 非 root）、docker compose |

## 快速开始

### 方式一：Docker Compose 一键启动（推荐）

```bash
docker compose up -d --build
# app          http://localhost:8080
# Prometheus   http://localhost:9090
# Grafana      http://localhost:3000  (admin/admin, 已预置数据源与 tinyURL 看板)
```

### 方式二：本地运行（需要本机已起 PostgreSQL 与 Redis）

```bash
cp .env.example .env          # 按需修改连接串
docker run -d --name pg  -e POSTGRES_USER=tinyurl -e POSTGRES_PASSWORD=tinyurl \
  -e POSTGRES_DB=tinyurl -p 5432:5432 postgres:16-alpine
docker run -d --name redis -p 6379:6379 redis:7-alpine

go run ./cmd/server           # 首次启动自动建表
```

## API

### 创建短链

```bash
curl -X POST http://localhost:8080/shorten \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://example.com"}'
# 201
# {"short_code":"100001","short_url":"http://localhost:8080/s/100001"}
```

错误场景：非 http(s) / 指向内网地址（如 `http://127.0.0.1`、`http://169.254.169.254`）
返回 `400`；同一 IP 超过令牌桶速率返回 `429`。

### 跳转

```bash
curl -i http://localhost:8080/s/100001
# HTTP/1.1 302 Found
# Location: https://example.com
```

短码不存在或格式非法返回 `404`。

### 其他

| 路径 | 说明 |
|---|---|
| `GET /healthz` | 存活探针（compose healthcheck 使用） |
| `GET /metrics` | Prometheus 指标端点 |

## 指标说明

| 类型 | 指标 | 用途 |
|---|---|---|
| Histogram | `shorturl_encode_duration_seconds{result}` | Base62 编码耗时，指数桶 0.5ms~2s，算 P99 |
| Counter | `shorturl_resolve_total{state}` | `cache_hit` / `db_hit` / `not_found`，命中率与穿透率由此推导 |
| Gauge | `shorturl_goroutines` | goroutine 数量（15s 刷新） |
| Gauge | `shorturl_redis_pool_usage_ratio` | Redis 连接池在用比例 |

命中率 / 穿透率 / P99 的 PromQL 见 [docs/benchmark.md](docs/benchmark.md)，
Grafana 看板已内建（文件夹 tinyURL → tinyURL Overview）。

## 测试与压测

```bash
go test ./...                  # 单元测试：Base62、缓存(miniredis)、限流、Service cache-aside
go vet ./cmd/... ./internal/... ./pkg/...
```

压测方法、hey 命令与结果记录表见 **[docs/benchmark.md](docs/benchmark.md)**
（脚本 `scripts/benchmark.sh`）。压测完成后把真实 QPS / P99 填进报告与简历。

## 配置项

全部通过环境变量注入（见 `.env.example`）：`DATABASE_URL`、`REDIS_ADDR`、
`CACHE_TTL`（默认 `1h`）、`NEGATIVE_CACHE_TTL`（默认 `1m`）、
`RATE_LIMIT_RPS`（默认 `10`）、`RATE_LIMIT_BURST`（默认 `5`）、
`TRUSTED_PROXIES`、`BASE_URL` 等。

## 项目结构

```text
.
├── cmd/server/main.go          # 入口：装配、超时、优雅停机、启动迁移
├── internal/
│   ├── config/                 # 环境变量 → 类型化配置
│   ├── model/                  # Link 模型 + ErrNotFound
│   ├── repo/                   # PostgreSQL 存储层 + migrations（go:embed）
│   ├── cache/                  # Redis cache-aside（正/负缓存、降级）
│   ├── service/                # Core：取号、Base62、校验、缓存编排
│   ├── api/                    # Handler / Router / 按 IP 限流中间件
│   └── metrics/                # Prometheus 指标定义与运行时采集
├── pkg/base62/                 # 定长 6 位 Base62 编解码（含单测）
├── deploy/
│   ├── prometheus/             # scrape 配置
│   └── grafana/                # 数据源 + 看板自动 provisioning
├── scripts/benchmark.sh        # hey 压测脚本
├── docs/benchmark.md           # 压测方法与报告
├── Dockerfile                  # 多阶段构建
└── docker-compose.yml          # app + pg + redis + prometheus + grafana
```

## License

MIT © 2026 GenJi
