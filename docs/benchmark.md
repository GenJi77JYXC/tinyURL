# 压测报告（Day 5）

> 所有数字均为真实实测，严禁虚构；压测机与命令在下方明确。

## 1. 测试环境

| 项 | 值 |
|---|---|
| 压测机 | Windows 11，Intel Core i9-14900HX（32 核），本机实测 |
| 部署方式 | `docker compose up`（app + PostgreSQL 16 + Redis 7） |
| 服务配置 | GOMAXPROCS 默认；连接池 PG 25 / Redis 20；CACHE_TTL=1h；GIN_MODE=release |
| 压测工具 | [hey v0.1.5](https://github.com/rakyll/hey)（通过 `go install` 获取），脚本：`scripts/benchmark.sh` |

## 2. 核心结果

### 2.1 跳转接口（缓存命中，302）

```bash
hey -disable-redirects -c 50 -z 30s http://192.168.5.51:8080/s/100001
```

| 指标 | 实测值 |
|---|---|
| 请求数 | **724,172**（30 秒） |
| **平均 QPS** | **约 24,100**（hey 在 Windows 上 Total 统计有 bug，按请求数/30s 换算） |
| P50 | 1.9 ms |
| P90 | 3.1 ms |
| **P99** | **4.9 ms** |
| 成功率 | 100%（全部为 302） |

### 2.2 不存在短码（负缓存防穿透，404）

```bash
hey -disable-redirects -c 50 -z 30s http://192.168.5.51:8080/s/zzzzzz
```

| 指标 | 实测值 |
|---|---|
| 请求数 | 711,056（30 秒） |
| **平均 QPS** | **约 23,700** |
| P99 | 4.8 ms |
| 成功行为 | 100% 返回 404（负缓存生效，仅首次查库） |

### 2.3 冷启动缓存回源

删除 Redis 键后立即请求，DB 命中并自动回写缓存：

| 指标 | 实测值 |
|---|---|
| 冷启动（缓存淘汰后首个请求） | P90 3.0ms，P99 5.0ms（23662 请求/1s） |
| 回写后热路径 | P90 3.0ms，P99 5.1ms（23900 请求/1s） |
| 回源代价 | 几乎不可见（PG 连接池 + 预编译索引覆盖良好） |

### 2.4 运行时指标（压测中采集）

| 指标 | 实测值 |
|---|---|
| `shorturl_resolve_total{state="cache_hit"}` | 1,481,173（含此前累计） |
| `shorturl_resolve_total{state="db_hit"}` | 22 |
| `shorturl_resolve_total{state="not_found"}` | 711,058 |
| 缓存命中率 | **99.9985%**（1,481,173 / (1,481,173 + 22)） |
| `shorturl_goroutines` | 79（50 并发压测峰值） |
| `shorturl_redis_pool_usage_ratio` | 0.85（50 并发持续打满） |

## 3. 单元微基准（已实测）

纯 CPU 的 Base62 编码，不依赖 DB/Redis：

```text
goos: windows / goarch: amd64
cpu: Intel(R) Core(TM) i9-14900HX
BenchmarkEncode-32   341,512,558   7.366 ns/op
```

复现：

```bash
go test -run='^$' -bench='Encode$' -benchtime=2s ./pkg/base62/
```

## 4. 服务级压测步骤（复现）

```bash
# 1. 启动全栈（PostgreSQL + Redis + app + Prometheus + Grafana）
docker compose up -d --build

# 2. 造一条短链
curl -s -X POST http://localhost:8080/shorten \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://example.com"}'
# -> {"short_code":"100001","short_url":"http://localhost:8080/s/100001"}

# 3. 跳转压测（缓存命中路径，30s / 50 并发）
hey -disable-redirects -c 50 -z 30s http://localhost:8080/s/100001

# 4. 不存在短码（验证空值缓存防穿透 + 404 路径）
hey -disable-redirects -c 50 -z 30s http://localhost:8080/s/zzzzzz

# 5. 生成接口压测（注意默认 10 QPS/IP 限流，测裸写吞吐时先调大 RATE_LIMIT_RPS）
RATE_LIMIT_RPS=10000 docker compose up -d --build app
hey -c 10 -z 10s -m POST -T application/json \
  -d '{"url":"https://example.com"}' http://localhost:8080/shorten
```

## 5. 观测口径

压测期间在 Grafana（http://localhost:3000，看板 tinyURL Overview）或 Prometheus UI（http://localhost:9090）观察：

- 缓存命中率：

  ```promql
  sum(rate(shorturl_resolve_total{state="cache_hit"}[1m]))
  / sum(rate(shorturl_resolve_total[1m]))
  ```

- 缓存穿透率（not_found 占比）：

  ```promql
  sum(rate(shorturl_resolve_total{state="not_found"}[1m]))
  / sum(rate(shorturl_resolve_total[1m]))
  ```

- 编码 P99：

  ```promql
  histogram_quantile(0.99,
    sum by (le) (rate(shorturl_encode_duration_seconds_bucket[1m])))
  ```
