# TinyURL 项目学习笔记

> 配套代码阅读指南。按「请求生命周期 → 各层详解 → 关键概念 → 工具与网络 → 面试问答」组织。
> 建议对照源码逐节阅读，文中的文件路径在 IDE 里都可以直接跳转。

## 0. 一张图看懂项目

```text
客户端
  │ POST /shorten {"url":"https://..."}
  ▼
┌─────────────────────────── Gin (:8080) ────────────────────────────┐
│  限流中间件（x/time/rate 令牌桶，按 IP，10 QPS + 突发 5）           │
│      │                                                             │
│      ▼                                                             │
│  Handler（参数绑定、错误码映射）                                    │
│      │                                                             │
│      ▼                                                             │
│  Service（核心业务逻辑，与 HTTP 无关）                              │
│      ├─ ValidateLongURL：scheme 白名单 + DNS 解析 + 内网 IP 拦截    │
│      ├─ repo.NextID：PG 序列取号（nextval）                         │
│      ├─ base62.Encode：id + 62^5 → 定长 6 位短码                    │
│      ├─ repo.InsertLink：写入 PostgreSQL                            │
│      └─ cache.SetLink：预热 Redis（失败仅告警）                     │
└────────────────────────────────────────────────────────────────────┘

客户端
  │ GET /s/{code}
  ▼
  Handler.Redirect
      │
      ▼
  Service.Resolve（cache-aside）：
      ① Redis 正缓存命中 → 302（cache_hit）
      ② Redis 负缓存命中 → 404（not_found，防穿透）
      ③ 缓存 miss/故障 → 查 PostgreSQL
           ├─ 存在 → 回写 Redis（TTL 1h）→ 302（db_hit）
           └─ 不存在 → 写负缓存（TTL 1m）→ 404
```

配套组件：`/metrics`（Prometheus 抓取）→ Grafana 看板；`/healthz`（compose 健康检查）。

---

## 1. 分层架构：每层只做一件事

| 层 | 目录 | 职责 | 依赖谁 |
|---|---|---|---|
| 入口装配 | `cmd/server/main.go` | 加载配置、初始化依赖、启动/停机 | 所有下层 |
| HTTP 层 | `internal/api/` | 路由、参数绑定、限流、状态码 | service |
| 业务层 | `internal/service/` | 校验、编码、缓存编排 | repo、cache（经接口） |
| 存储层 | `internal/repo/` | PostgreSQL 读写、迁移 | database/sql |
| 缓存层 | `internal/cache/` | Redis 读写、负缓存 | go-redis |
| 基础设施 | `internal/config/`、`internal/metrics/`、`internal/model/` | 配置收口、指标定义、共享模型 | — |
| 纯算法 | `pkg/base62/` | 定长 Base62 编解码 | 无 |

**关键原则：依赖方向单向（上→下），业务层通过接口依赖存储/缓存（Port-Adapter 模式）**，所以单测可以用 fakeStore + miniredis 替换真实依赖。

---

## 2. 各层要点速查

### 2.1 Base62 编码（`pkg/base62/base62.go`）

- 字母表 `0-9a-zA-Z` 共 62 个 URL-safe 字符，6 位空间 = 62⁶ ≈ 568 亿。
- **偏移 62⁵**（MinID）：所有 id 编码前先加 916,132,832，保证输出**恒定 6 位**（id=1 → `100001` 而不是 `1`）。可用容量 62⁶−62⁵ ≈ 558 亿。
- Encode：从数组末尾倒着填（取模先得低位），天然定长、免反转。
- Decode：128 字节 ASCII 查表（非法字符=255），O(1)/字符。
- 实测性能 **7.4 ns/op**（BenchmarkEncode）。
- 陷阱：字母表里小写（10-35）在大写（36-61）之前，码值**数值递增 ≠ ASCII 字典序递增**——测试断言用"无重复"，不能用字符串大小比较。

### 2.2 配置层（`internal/config/config.go`）

- 一次解析、类型收口、启动校验（fail-fast），替代散落各处的 `os.Getenv`。
- `getenv/getint/getfloat/getdur` 四个解析函数；RPS 用 float64 因为 `rate.Limit` 底层是浮点。
- 校验集中在启动时：配置不合法直接退出，不带病运行。
- `.env` 只是本地便利（godotenv），容器里靠环境变量注入；**.env 不进 git**（密钥安全）。

### 2.3 存储层（`internal/repo/postgres.go` + `migrations/`）

- **先取号再插入**：`SELECT nextval('links_id_seq')` 预分配 ID → 编码 → 一次 INSERT。
  旧版"先插空 code 再 UPDATE 回填"有两趟往返 + UNIQUE 冲突 bug。
- nextval 非事务：INSERT 失败会留下 ID gap——对短链场景无害。
- 连接池：MaxOpen 25 / MaxIdle 10 / MaxLifetime 30m / MaxIdleTime 5m（轮换防网络僵死）。
- `short_code CHAR(6) UNIQUE`：跳转查询走唯一索引。
- 迁移用 `//go:embed` 把 SQL 编进二进制，启动时自动执行（幂等：`IF NOT EXISTS`）。

### 2.4 缓存层（`internal/cache/redis.go`）

- key 设计：`short:{code}` → URL；**负缓存**同名 key 存空串，TTL 1 分钟。
- **三态查询** StateMiss / StateHit / StateNegative——区分"没有 key"和"key 存在但值为空（已知不存在）"。
- 防穿透：不存在的短码第一次查库后写负缓存，后续 1 分钟内直接 404，不打 DB。
- Redis 是**软依赖**：构造不 ping，操作出错返回 error 由上层降级；带连接超时参数防慢查询拖死请求。
- 测试用 `miniredis`（纯 Go 内存版 Redis），可断言 TTL 和 key 存在性。

### 2.5 业务层（`internal/service/`）

**shortener.go**
- 接口隔离：`LinkStore` / `LinkCache` 两个小接口，测试可注入假实现。
- Shorten：校验 → 取号 → 编码（计时埋点 Histogram）→ 落库 → 预热缓存（失败仅告警）。
- Resolve：格式预检 → 正缓存 → 负缓存 → 查库 → 回写/负缓存。每步都有降级路径，Redis 挂了整个服务仍然可用。

**validate.go（SSRF 防护）**
- 链路：长度限制 → url.Parse → scheme 白名单（http/https）→ **DNS 解析** → 逐个 IP 检查。
- 拦截：loopback / 私网（RFC1918）/ 链路本地（含云元数据 169.254.169.254）/ 多播 / CGNAT（100.64.0.0/10，Go 旧版 IsPrivate 不含）。
- 为什么必须 DNS 解析：攻击者可把 `evil.com` 解析到内网 IP，只看域名拦不住。
- 真实踩坑：`netip.Addr.As4()` 对 IPv6 会 panic——必须先 `Is4()` 判断（压测时发现，已加回归测试）。

### 2.6 指标层（`internal/metrics/metrics.go`）

三类指标对应"延迟 / 吞吐 / 饱和度"：

| 类型 | 指标 | 设计要点 |
|---|---|---|
| Histogram | `shorturl_encode_duration_seconds` | **指数桶** 0.5ms 起 ×2 共 12 桶；只测编码纯 CPU 耗时，不含 IO |
| Counter | `shorturl_resolve_total{state}` | label 区分 cache_hit / db_hit / not_found；命中率、穿透率由 PromQL 推导 |
| Gauge | `shorturl_goroutines`、`shorturl_redis_pool_usage_ratio` | 后台 goroutine 每 15s 刷新；必须监听 ctx.Done() 退出防泄漏 |

常用 PromQL：

```promql
# 缓存命中率
sum(rate(shorturl_resolve_total{state="cache_hit"}[1m])) / sum(rate(shorturl_resolve_total[1m]))
# 编码 P99
histogram_quantile(0.99, sum by (le) (rate(shorturl_encode_duration_seconds_bucket[1m])))
```

### 2.7 HTTP 层（`internal/api/`）

- 状态码语义：创建成功 **201**；跳转 **302**（不是 301——301 会被浏览器永久缓存，后续点击不经过服务，统计和灰度都没法做）；限流 **429**；校验失败 **400**；短码不存在 **404**。
- 路由：`/s/:code` 而不是旧版的 `/:short`——避免 catch-all 与 `/metrics`、`/healthz` 冲突。
- `/metrics` 不挂限流（否则 Prometheus 自己会吃到 429）。
- 限流：`x/time/rate` 令牌桶，**每 IP 独立桶**，默认 10 QPS + 突发 5；后台 sweeper 每 1 分钟清理 10 分钟未活跃的 IP 条目，防 map 无限增长。
- `SetTrustedProxies`：反代后获取真实客户端 IP 必须配，否则限流针对的是 Nginx 的 IP。

### 2.8 入口（`cmd/server/main.go`）

- main 只做装配：config → repo → cache → metrics → service → handler → router → http.Server。
- **优雅停机**：`signal.NotifyContext` 捕获 SIGINT/SIGTERM → `srv.Shutdown()` 等存量请求完成 → 关闭 sweeper、连接池。
- **HTTP 超时四件套**：ReadHeader 5s / Read 10s / Write 10s / Idle 60s——防慢客户端拖死服务。
- 硬依赖（PG）连不上 fail-fast；软依赖（Redis）连不上降级运行。

### 2.9 Docker 化

- **多阶段构建**：builder 阶段编译（含 Go 工具链），运行阶段 alpine + 仅二进制，镜像从 ~500MB 缩到 ~35MB。
- `CGO_ENABLED=0 -trimpath -ldflags="-s -w"`：静态编译、去符号表。
- 非 root 用户运行；HEALTHCHECK 用 wget 打 `/healthz`。
- `.dockerignore`：排除 .git、测试文件、.env（防敏感信息进构建上下文）。
- compose：`depends_on: condition: service_healthy` 等 PG/Redis 真正就绪再启动 app；Prometheus/Grafana 数据源与看板全部 provisioning 自动化。

---

## 3. 网络知识补充（本次踩坑记录）

### 3.1 为什么 127.0.0.1:8080 不通、192.168.x.x:8080 通

- compose 的 `8080:8080` 映射让 Docker 代理绑定 `0.0.0.0:8080`（所有网卡的兜底）。
- Steam 精确绑定了 `127.0.0.1:8080`。**精确 IP 绑定优先于通配符绑定** → loopback 流量被 Steam 截走。
- 物理网卡 IP 没有精确绑定 → 落到 `0.0.0.0` 兜底 → 到达 Docker。
- 本机调试只想自己用：`"127.0.0.1:8080:8080"` 绑定 loopback，局域网访问不到（数据库端口推荐这样做）。

### 3.2 如何找主网卡 IP

```powershell
Get-NetRoute -DestinationPrefix "0.0.0.0/0" | Select-Object -First 1 |
  ForEach-Object { Get-NetIPAddress -InterfaceIndex $_.InterfaceIndex -AddressFamily IPv4 }
```

有默认路由（能上网）的接口对应的 IP 才是主网卡 IP。

### 3.3 example.com 的"该域名仅用于文档示例"

RFC 2606 保留的文档占位域名，IANA 挂的说明页。302 跳转到它就证明链路完整。压测务必加 `-disable-redirects`，否则几万个请求会真的打向目标网站。

---

## 4. 工具速查

### hey（HTTP 压测）

```powershell
& "$env:USERPROFILE\go\bin\hey.exe" -disable-redirects -c 50 -z 10s "http://<IP>:8080/s/<code>"
```

- `-c`：并发 worker 数；`-z`：持续时间；`-n`：总请求数（二选一）
- `-disable-redirects`：测跳转服务必须加
- `-m POST -T application/json -d '{"url":"..."}'`：压写接口
- Windows 下长测试的 Total/Requests/sec 统计有 bug，用「请求数 ÷ 秒数」换算

### 常用验证命令

```powershell
# 全量测试（不依赖真实 PG/Redis）
go test ./...
# 静态检查
go vet ./cmd/... ./internal/... ./pkg/...
# 编码微基准
go test -run='^$' -bench='Encode$' -benchtime=2s ./pkg/base62/
# 起停
docker compose up -d --build    # 起全栈
docker compose down             # 停（-v 连数据一起清）
docker logs tinyurl-app --tail 30
# 手动操作 Redis
docker exec tinyurl-redis redis-cli KEYS 'short:*'
docker exec tinyurl-redis redis-cli DEL short:100001   # 强制回源演示
```

---

## 5. 面试问答预备

**Q：短码怎么生成的？会冲突吗？**
A：PostgreSQL BIGSERIAL 序列 `nextval` 取号（原子、无冲突），加上 62⁵ 偏移后做 Base62 编码得到定长 6 位短码，容量约 558 亿。取号失败留 gap 但无害。相比 hash 截断方案，序列方案天然无碰撞、无重试。

**Q：跳转链路怎么设计的？**
A：cache-aside：Redis 命中直接 302；miss 查 PG 回写（TTL 1h）；不存在写负缓存（TTL 1m）防穿透。Redis 是软依赖，故障时降级直查 PG。用 302 不用 301，保证每次点击经过服务以便统计和灰度。

**Q：怎么防刷、防滥用？**
A：`x/time/rate` 按 IP 令牌桶（10 QPS + 突发 5）只挂写接口；URL 校验做 scheme 白名单 + DNS 解析后拦截内网 IP（防 SSRF 打云元数据）；限流 map 有后台清扫防内存泄漏。

**Q：监控怎么做的？**
A：Prometheus 三类指标：Histogram 看编码耗时分布（指数桶算 P99）；Counter 按 label 统计 cache_hit/db_hit/not_found 推导命中率与穿透率；Gauge 暴露 goroutine 数和 Redis 连接池使用率。Grafana 看板 provisioning 自动加载。

**Q：压测数据？**
A：i9-14900HX 本机 docker compose 全栈，hey 50 并发 30s：跳转 72.4 万请求、约 24k QPS、P99 4.9ms、100% 302；缓存命中率 99.99%+。

**Q：遇到过什么 bug？**
A（真实案例）：SSRF 校验里 `netip.Addr.As4()` 对 IPv6 地址 panic——它在 if 初始化语句里先于 Is4() 判定执行。压测 + 回归测试抓出并修复。另一个：旧版"先插空 code 再回填"的写法在第二条数据就触发 UNIQUE 冲突，新版改为 nextval 预取号根除。

---

## 6. 实测数据（简历素材）

| 场景 | QPS | P99 | 说明 |
|---|---|---|---|
| 跳转（缓存命中） | ~24,100 | 4.9ms | 30s 724,172 请求，100% 302 |
| 跳转（负缓存 404） | ~23,700 | 4.8ms | 防穿透生效 |
| 缓存回源 | ~23,900 | 5.0ms | 冷热路径几乎无差异 |
| Base62 编码 | — | 7.4 ns/op | 纯 CPU 微基准 |

环境：Windows 11 + i9-14900HX，docker compose（Go 1.25 + PG 16 + Redis 7）。
