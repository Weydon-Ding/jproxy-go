# AGENTS.md

## 先读

```text
AGENTS.md
README.md
docs/architecture.md
docs/milestones.md
docs/v0.1.0-plan.md
```

迁移相关任务还必须对照原版：

```text
..\jproxy
```

## 当前目标

```text
v0.1.0 = Core Proxy MVP
```

## 当前优先级

1. 补测试。
2. 完善 README / Docker 说明。
3. 做 v0.1.0 接入验收。

## 验证命令

```bat
go test ./...
go build ./cmd/jproxy
go vet ./...
golangci-lint run
```

说明：

```text
golangci-lint run 依赖本地已安装 golangci-lint。
```

## 重要路径

```text
cmd/jproxy                 程序入口
internal/config            配置
internal/cache             缓存
internal/proxy             代理与 XML 处理
configs/jproxy.env.example 环境变量示例
deployments/docker         Docker
docs/architecture.md       架构
docs/milestones.md         里程碑
docs/v0.1.0-plan.md        v0.1.0 计划
```

## 分支

```text
main        发布
 dev        开发
feature/*   功能
bugfix/*    修复
hotfix/*    线上修复
release/*   预发布
```

发布流程：

```text
release/vX.Y.Z -> main
tag vX.Y.Z
main -> dev
```

## 提交

使用 Conventional Commits：

```text
feat: ...
fix: ...
test: ...
docs: ...
chore: ...
```

## Agent 规则

1. 执行前先读“先读”里的文档。
2. 迁移实现默认保持原版行为；有意改变必须写明原因并更新文档/测试。
3. 关键产出必须写入仓库文件，不只留在聊天里。
4. 修改完成后自行提交，提交信息使用 Conventional Commits。
5. 返回 commit hash、修改摘要、验证结果和风险。
6. 没执行的测试不要写成已通过。
7. 修改后尽量运行最小验证。
