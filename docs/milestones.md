# jproxy-go 里程碑与待办

## 分支策略

- `main`：外部发布版本。
- `dev`：内部开发版本。
- `feature/*`：从 `dev` 分出，用于功能开发，完成后合并回 `dev`。
- `bugfix/*`：用于修复 `dev` 或 `release` 阶段发现的问题。
- `hotfix/*`：从 `main` 分出，用于修复外部发布版本问题。
- `release/*`：从 `dev` 分出，用于预发布测试；测试完成后合并到 `main`，发布完成后再由 `main` 合并回 `dev`。

提交信息遵循 Conventional Commits。

## M0：仓库基线

状态：已完成。

范围：

- 初始化 Go 工程结构。
- 初始化 Git 仓库。
- 建立 `main` / `dev` 分支。
- 完成 Initial commit。
- `go test ./...`、`golangci-lint run`、`go build ./cmd/jproxy` 通过。

## M1：核心代理 MVP

目标：稳定替代基础 Jackett/Prowlarr 代理链路。

范围：

- `/sonarr/jackett/*`
- `/sonarr/prowlarr/*`
- `/radarr/jackett/*`
- `/radarr/prowlarr/*`
- 请求转发。
- 结果缓存。
- offset 分页缓存。
- XML 合并与裁剪。
- 基础搜索词扩展。
- 补充单元测试。

建议分支：

```text
feature/proxy-mvp-tests
```

验收：

```bat
go test ./...
golangci-lint run
go build ./cmd/jproxy
```

## M2：规则格式化能力

目标：迁移 Java 版核心规则格式化能力。

范围：

- 分析 Java 版 `executeFormatRule`、`FormatUtil`、`XmlUtil`、RuleService 逻辑。
- 设计 Go 版规则模型。
- 支持从本地 JSON/配置文件加载规则。
- 实现 Sonarr/Radarr XML item 级处理。
- 补充典型规则测试用例。

建议分支：

```text
feature/rule-formatting
```

## M3：标题库 / 别名扩展

目标：补齐基于标题库的搜索扩展能力。

范围：

- Sonarr 标题库。
- Radarr 标题库。
- TMDB 标题别名。
- 本地文件或 SQLite 存储方案评估。
- 搜索词扩展测试。

建议分支：

```text
feature/title-library
```

## M4：运行配置与部署

目标：形成可部署服务。

范围：

- 配置文件与环境变量兼容。
- Dockerfile 完善。
- README 部署说明。
- 健康检查。
- 日志格式。
- 优雅关闭。

建议分支：

```text
feature/runtime-config
```

## M5：预发布测试

目标：进入 release 流程，验证 v0.1.0。

建议分支：

```text
release/v0.1.0
```

验收：

- 本地构建通过。
- Docker 构建通过。
- Jackett/Prowlarr 实测通过。
- Sonarr/Radarr 接入测试通过。
- README 可按步骤部署。

## M6：v0.1.0 发布

目标：发布第一个外部可用版本。

流程：

1. `release/v0.1.0` 合并到 `main`。
2. 在 `main` 打 tag：`v0.1.0`。
3. `main` 合并回 `dev`。

定位：

- 无 Web 管理后台。
- 不完整迁移 Java 数据库管理能力。
- 不包含完整规则格式化、标题库/别名库能力；这些能力继续放在 v0.2.0+ 迁移。
- 具备核心代理、基础搜索扩展、XML 合并/裁剪、缓存与可部署能力。
- 核心代理 MVP 范围内默认对齐原版 `../jproxy` 行为；有意差异必须写入文档和测试。

## 当前优先级

1. 开 `feature/proxy-mvp-tests`。
2. 补齐现有代理主链路测试。
3. 再进入 `feature/rule-formatting`。
