# 古籍文物库房环境异常处置与借展交接服务

单进程 Go HTTP JSON 服务，基于 `database/sql` 与嵌入式 SQLite（modernc.org/sqlite，纯 Go 无 CGO）持久化到 `DB_PATH` 文件，管理文物藏品、库房/展柜环境、异常处置、借展交接与归还验收。

## 功能总览

- **藏品管理**：登记、库位分配、保存等级、附件、受约束注销（保留审计），状态历史快照全程保留
- **环境监测**：传感器注册、温湿度采样上报、阈值规则版本管理（草稿/启用/退役）、迟到数据仅存档
- **异常处置**：环境巡检自动分级（轻微/严重/危急）、隔离保护、保护处置、复核关闭，全程真实事务
- **借展交接**：申请、审批（冻结藏品状态/保存等级/包装清单/审批规则快照）、出库清点+首段交接（同事务）、追加交接、运输节点、展陈确认、归还清点、归还验收与借展关闭（同事务）
- **专题查询**：临近借展仍有环境异常的藏品、库房风险排序、传感器连续越界、包装清单差异、逾期未归还借展
- **后台作业**：环境巡检、借展到期、交接超时，失败指数退避重试，进程重启自动恢复
- **工程能力**：统一错误格式、键集稳定分页、幂等键去重、乐观锁并发控制、结构化 JSON 日志、优雅关闭、可注入时钟

## 快速开始

```bash
# 本地运行（Go 1.26+，GOTOOLCHAIN=local）
export GOTOOLCHAIN=local
PORT=8080 DB_PATH=./data/app.db go run ./cmd/server

# 健康检查
curl http://localhost:8080/healthz
```

## 配置项

| 环境变量 | 默认 | 说明 |
| ---- | ---- | ---- |
| `PORT` | `8080` | HTTP 监听端口 |
| `DB_PATH` | （必填） | SQLite 文件路径，禁止 `:memory:` |
| `LOG_LEVEL` | `info` | 日志级别 debug/info/warn/error |
| `PRE_LOAN_WINDOW_HOURS` | `72` | 借展前置环境合格窗口（小时） |
| `ENV_MAX_GAP_MINUTES` | `60` | 窗口内允许的最大采样间隔（分钟） |
| `HANDOVER_TIMEOUT_HOURS` | `48` | 交接超时阈值（小时） |
| `JOB_INTERVAL_SECONDS` | `5` | 后台作业调度周期（秒） |
| `JOB_MAX_ATTEMPTS` | `5` | 作业最大尝试次数 |

## 分层架构

```
cmd/server            入口：配置加载、日志、HTTP 服务、优雅关闭
internal/config       配置（环境变量）
internal/clock        可注入时钟（Real/Fake）
internal/domain       领域模型、状态枚举、分页、领域错误
internal/rules        纯函数规则：阈值判定、异常分级、前置窗口、交接有序性
internal/repository   持久化接口（Querier 抽象 *sql.DB 与 *sql.Tx）
internal/sqlite       SQLite 连接与 schema 迁移
internal/sqliterepo   仓储 SQLite 实现
internal/tx           真实 SQLite 事务管理（提交/回滚）
internal/audit        审计与藏品状态快照记录
internal/service      业务服务：藏品/环境/异常/借展/清点/交接/归还/查询
internal/httpx        HTTP 层：路由、统一错误、分页、中间件、健康检查
internal/jobs         后台作业：巡检、借展到期、交接超时、失败重试、重启恢复
internal/testenv      测试脚手架（真实临时 SQLite 文件 + 假时钟）
```

## 文档

- [领域说明](docs/01-领域说明.md)
- [状态转换表](docs/02-状态转换表.md)
- [数据模型](docs/03-数据模型.md)
- [接口契约](docs/04-接口契约.md)

## 接口示例

```bash
# 登记藏品
curl -X POST localhost:8080/api/v1/artifacts -d '{"code":"WJ-0001","name":"宋刻本","level_id":1}'

# 上报环境采样
curl -X POST localhost:8080/api/v1/env-samples -d '{"sensor_id":1,"temperature":20.5,"humidity":50}'

# 借展审批（冻结快照）
curl -X POST localhost:8080/api/v1/loans/1/approve -d '{"version":2,"reviewer":"赵六"}'

# 库房风险排序
curl localhost:8080/api/v1/queries/warehouses/risk-ranking
```

## 测试与校验

```bash
export GOTOOLCHAIN=local
go test ./...
go vet ./...
go build ./...
```

测试全部使用真实临时 SQLite 文件，覆盖借展全链、环境时间窗、历史快照、附件清点、交接顺序、幂等、乐观锁、事务回滚、稳定分页、作业重试与重启恢复。

## Docker 构建（linux/amd64、linux/arm64）

```bash
./build_docker.sh hwj-go-107:amd64 linux/amd64
./build_docker.sh hwj-go-107:arm64 linux/arm64
```

镜像固定使用 `golang@sha256:53eeac89074db483fdf0ab3be1df32bf6e47562263d2d0d6baa7f26acb4957dd`，架构无关，`GOTOOLCHAIN=local`，go.mod 语言版本 `go 1.21`。容器内验证：

```bash
go test ./... && go vet ./... && go build ./...
DB_PATH=/tmp/app.db PORT=8080 go run ./cmd/server   # 无网络亦可运行
```

评测环境说明见 [BENZHI_README.md](BENZHI_README.md)。
