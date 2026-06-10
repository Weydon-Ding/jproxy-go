# v0.1.0 Release Note 草案与发布 Checklist

> 状态：**beta candidate / 待验证**  
> 版本定位：**Core Proxy MVP**  
> 重要说明：v0.1.0 不是原版 Java `jproxy` 的完整替代，仅承诺核心代理 MVP 范围内的原版行为对齐。迁移实现默认对照 `../jproxy`；超出 MVP 的 Java 版能力在本版本明确列为范围外。

## 1. 版本范围

v0.1.0 的目标是交付第一个可部署、可真实接入测试的 Go 版核心代理测试版，用于验证：

- Sonarr/Radarr 到 Jackett/Prowlarr 的基础代理链路。
- 基础搜索词扩展、XML 合并/计数/裁剪。
- 结果缓存与 offset 分页缓存。
- 环境变量配置、健康检查、本地二进制与 Docker 部署路径。

### 范围内

| 能力 | 范围说明 |
|---|---|
| Sonarr/Radarr 路由 | 支持 `/sonarr/jackett/*`、`/sonarr/prowlarr/*`、`/radarr/jackett/*`、`/radarr/prowlarr/*`。 |
| Jackett/Prowlarr 代理 | 去除 jproxy 前缀后转发到对应上游，保留原始路径与 query 参数。 |
| 基础搜索扩展 | Sonarr 基础去尾部集数；Radarr 保留原标题并尝试去年份。 |
| XML 处理 | RSS/Torznab XML item 计数、合并、按 `limit` 裁剪。 |
| 缓存 | 结果缓存、offset 分页缓存；结果缓存 key 忽略 `apikey` 但请求仍透传。 |
| 配置 | 通过环境变量配置监听地址、上游地址、缓存 TTL、最小结果数、HTTP timeout。 |
| 健康检查 | `/health` 返回 HTTP 200 和 `ok`。 |
| 部署文档 | README 包含本地运行、Docker build/run、Compose、Sonarr/Radarr 接入路径。 |
| QA 文档 | `docs/v0.1.0-qa.md` 给出测试矩阵、手工接入步骤、阻塞项与当前验证记录。 |

### 范围外

| 能力 | v0.1.0 处理方式 |
|---|---|
| Web 管理后台 | 不包含。 |
| 登录 / JWT | 不包含。 |
| SQLite / Java DB 完整兼容 | 不包含；后续版本评估。 |
| 完整规则格式化引擎 | 不包含；仅保留后续扩展方向。 |
| 标题库 / 别名库 / TMDB 同步 | 不包含完整能力；仅有基础搜索词扩展。 |
| Downloader 集成 | 不包含 qBittorrent/Transmission 代理与管理能力。 |
| 多用户、多实例管理 | 不包含。 |

## 2. 已实现内容

- Core Proxy MVP 四类入口路径：
  - `/sonarr/jackett/*`
  - `/sonarr/prowlarr/*`
  - `/radarr/jackett/*`
  - `/radarr/prowlarr/*`
- 上游 Jackett/Prowlarr URL 拼接与 query 参数透传。
- 上游不可达或非 2xx 时返回错误，不应导致进程崩溃。
- 基础搜索词扩展：
  - Sonarr：对带尾部集数的搜索词做基础简化。
  - Radarr：对带年份的搜索词保留原标题并尝试去掉年份。
- RSS/Torznab XML：
  - 统计 `<item>` 数量。
  - 合并多次上游响应。
  - 按 `limit` 裁剪，覆盖 Prowlarr 分页异常场景。
  - 空 XML / 无 item XML 不应 panic。
- 结果缓存和 offset 分页缓存。
- 环境变量配置与默认值。
- `/health` 健康检查。
- Dockerfile、Compose 示例与 README 部署说明。
- QA 验收清单与本 release note/checklist 草案。

## 3. 原版 jproxy 对照摘要

本版本已按 MVP 口径对照原版 `../jproxy` 的核心代理行为：

| 对照点 | 原版 Java jproxy | Go 版 v0.1.0 状态 |
|---|---|---|
| Filter 路径 | `Common.FILTER_SONARR_JACKETT_PATH` 等四类路径 | 使用相同四类 jproxy 前缀。 |
| 上游路径拼接 | `getIndexerUrl` 去除 `/sonarr|radarr/jackett|prowlarr` 前缀后拼接上游 | Go 版执行同类前缀剥离与上游拼接。 |
| Query 处理 | `executeNewRequest` 带 query 请求上游 | Go 版保留 query 参数并请求上游；特殊字符编码仍需真实上游确认。 |
| 结果缓存 key | 原版 `generateCacheKey` 忽略 `apikey` | Go 版缓存 key 忽略 `apikey`，请求仍透传。 |
| offset 缓存 | 原版 `INDEXER_SEARCH_OFFSET` 缓存 offset 列表 | Go 版内存 offset cache 实现同类用途。 |
| offset 当前索引 | 原版边界使用 `>=` | Go 版保持 `>=` 边界行为。 |
| XML 合并/裁剪 | 原版 `XmlUtil.merger` 与分页裁剪 | Go 版实现 `mergeXML` / `trimXML`，单测覆盖；复杂真实 XML 待样例回归。 |
| 格式化规则 | 原版会执行完整规则格式化 | v0.1.0 明确不包含，是已知限制。 |
| 标题库/别名库 | 原版依赖 DB/服务查询 | v0.1.0 不包含完整能力，是已知限制。 |
| 配置来源 | 原版 DB/system config + application 配置 | v0.1.0 使用环境变量，是部署差异。 |

## 4. 不兼容 / 已知限制

- **不是完整 Java jproxy 替代**：v0.1.0 仅覆盖 Core Proxy MVP。
- **规则格式化缺失**：不会执行原版完整 `executeFormatRule`、`FormatUtil`、RuleService 等规则格式化能力。
- **无 Java DB 兼容**：不读取原版 SQLite/Java DB，不迁移 system config、标题库、别名库、规则库。
- **搜索扩展能力有限**：仅提供基础关键词变体，不等同于原版标题库/别名库驱动的扩展。
- **配置方式不同**：当前以环境变量为主，不提供 Web UI 配置。
- **Docker build/run 未验证**：当前 Docker 环境由用户明天解决，不能标记为通过。
- **真实接入未完成**：Jackett/Prowlarr、Sonarr/Radarr 的真实集成验收仍是测试版发布前关键门禁。
- **复杂 XML 兼容性待增强**：已有单测覆盖基础 XML 合并/裁剪，但仍建议补充真实 Jackett/Prowlarr RSS 样例回归。

## 5. 验证记录

### 当前已记录的最小验证

详见 `docs/v0.1.0-qa.md`。当前记录如下：

| 验证项 | 状态 | 说明 |
|---|---|---|
| `go test ./...` | 已通过 | 来自当前 QA 记录；本次文档更新后也应重新执行。 |
| `go build ./cmd/jproxy` | 已通过 | 来自当前 QA 记录；本次文档更新后也应重新执行。 |
| `go vet ./...` | 已通过 | 来自当前 QA 记录；本次文档更新后也应重新执行。 |
| `golangci-lint run` | 已通过 | 来自当前 QA 记录；依赖本地已安装。 |
| Docker build | **待验证** | 当前 Docker 环境由用户明天解决，不能标记通过。 |
| Docker run + `/health` | **待验证** | 当前 Docker 环境由用户明天解决，不能标记通过。 |
| Jackett/Prowlarr 真实搜索 | 待验证 | 需要至少一组真实上游搜索。 |
| Sonarr/Radarr UI 接入 | 待验证 | 需要至少一组通过 jproxy-go 的 UI 测试与实际搜索。 |

### 本 release note/checklist 更新后的建议验证命令

```bat
go test ./...
go build ./cmd/jproxy
go vet ./...
golangci-lint run
```

未执行或环境不可用的命令必须保留为“未执行/待验证”，不得写成已通过。

## 6. 测试版阻塞项

| 编号 | 阻塞项 | 影响 | 优先级 | 退出条件 |
|---|---|---|---|---|
| B-001 | Docker build/run 当前未验证 | Docker 是 v0.1.0 范围内能力，未验证会影响部署可信度。 | P0 | 用户修复 Docker 环境后，`docker build` 成功，容器启动后 `/health` 返回 `ok`。 |
| B-002 | 缺少真实 Jackett/Prowlarr 搜索验证 | 无法证明上游代理链路在真实服务中可用。 | P0 | 至少 Jackett 或 Prowlarr 一组真实搜索通过；建议两者都验证。 |
| B-003 | 缺少 Sonarr/Radarr UI 接入验证 | 无法证明外部客户端接入流程可用。 | P0 | 至少 Sonarr 或 Radarr 一组索引器 Test 和实际搜索通过；建议两者都验证。 |
| B-004 | 复杂真实 XML 样例回归不足 | XML 合并/裁剪对真实格式存在兼容风险。 | P1 | 收集典型 Jackett/Prowlarr RSS 样例并完成手工或自动回归。 |

## 7. 发布前 Checklist

### 代码与静态检查

- [ ] `go test ./...` 通过。
- [ ] `go build ./cmd/jproxy` 通过。
- [ ] `go vet ./...` 通过，或记录风险并接受。
- [ ] `golangci-lint run` 通过；若本地未安装，记录为未执行。
- [ ] 工作区无未提交的业务/文档变更。

### 本地运行与健康检查

- [ ] 本地 `go run ./cmd/jproxy` 可启动。
- [ ] `/health` 返回 HTTP 200 和 `ok`。
- [ ] 非声明路径返回 404。
- [ ] 上游不可达返回 502，进程不崩溃。

### 核心代理与原版行为对齐

- [ ] `/sonarr/jackett/*` 能转发到 Jackett 对应路径。
- [ ] `/sonarr/prowlarr/*` 能转发到 Prowlarr 对应路径。
- [ ] `/radarr/jackett/*` 能转发到 Jackett 对应路径。
- [ ] `/radarr/prowlarr/*` 能转发到 Prowlarr 对应路径。
- [ ] `apikey`、`t`、`q`、`cat`、`season`、`ep`、`limit`、`offset` 等 query 参数按预期透传或按分页逻辑更新。
- [ ] 结果缓存 key 忽略 `apikey`，但 `apikey` 仍透传上游。
- [ ] offset 边界行为保持原版 `>=` 逻辑。
- [ ] Radarr 年份扩展至少一组实测符合预期。
- [ ] Sonarr 集数简化至少一组实测符合预期。

### XML 与缓存

- [ ] 多次上游 XML 响应可合并为可用 RSS/Torznab XML。
- [ ] `limit` 裁剪后 item 数不超过请求限制。
- [ ] 空 XML / 无 item XML 不导致 panic。
- [ ] 结果缓存命中符合预期。
- [ ] 结果缓存 TTL 过期后会重新请求上游。
- [ ] offset 分页缓存至少两页请求符合预期。

### Docker 与部署

- [ ] Docker build 通过。**当前待验证，不能提前勾选。**
- [ ] Docker run 后 `/health` 返回 `ok`。**当前待验证，不能提前勾选。**
- [ ] Compose 示例可启动。**当前待验证，不能提前勾选。**
- [ ] README 本地运行步骤可按文档执行。
- [ ] README Docker 步骤在 Docker 环境修复后可按文档执行。
- [ ] README Sonarr/Radarr 接入 URL 与实际验证一致。

### 外部集成

- [ ] Jackett 至少一组真实搜索通过。
- [ ] Prowlarr 至少一组真实搜索通过。
- [ ] Sonarr 通过 jproxy-go 添加/测试索引器通过。
- [ ] Radarr 通过 jproxy-go 添加/测试索引器通过。
- [ ] Sonarr/Radarr 实际搜索返回可用结果，jproxy-go 不崩溃。

### 发布资料

- [ ] release note 写明 v0.1.0 是 Core Proxy MVP，不是 Java jproxy 完整替代。
- [ ] release note 写明范围内、范围外、不兼容/已知限制。
- [ ] release note 写明 Docker 和真实接入的验证状态。
- [ ] `docs/v0.1.0-qa.md` 与本文件的阻塞项状态一致。
- [ ] 发布 tag 前确认版本号、分支和目标 commit。

## 8. 测试版就绪判断

### 当前判断

当前状态建议为：**v0.1.0 beta candidate / 待验证**。

原因：

- README、Docker 使用说明、QA 验收清单和 release note/checklist 已补齐。
- Core Proxy MVP 的代码级最小验证已有通过记录。
- 但 Docker build/run 当前未验证，且真实 Jackett/Prowlarr、Sonarr/Radarr 接入尚未完成。

### 可宣布“测试版就绪”的最低门槛

只有当以下条件均满足时，才建议将状态改为 **v0.1.0 beta ready**：

1. `go test ./...`、`go build ./cmd/jproxy` 通过；建议 `go vet ./...` 通过。
2. Docker build 通过。
3. Docker run 后 `/health` 返回 `ok`。
4. 至少一组 Jackett 或 Prowlarr 真实搜索通过。
5. 至少一组 Sonarr 或 Radarr 通过 jproxy-go 的 UI 接入测试与实际搜索通过。
6. release note 与 QA 文档明确仍未覆盖的范围外能力和已知限制。

在 B-001、B-002、B-003 未关闭前，不建议发布为“测试版完全就绪”；可以发布为“测试版候选，等待 Docker 与真实接入验收”。
