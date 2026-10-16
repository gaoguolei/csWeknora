# 会话工作区项目验证

测试页按当前会话工作区识别项目、运行验证，并保存持久历史。前后端分离的仓库会按 `frontend`、`backend` 等项目目录分组；每个会话独立执行，不继承其他会话的通过结果。

## 使用

1. 在对话中拉取项目，打开右侧「测试」页。
2. 选择全部项目或某个项目，点击「运行项目验证」。也可以运行单个测试文件、Go 包或验证项。
3. 展开结果查看真实用例、失败断言、命令和日志。支持停止、重跑失败项，以及把失败证据回填到对话让 AI 分析或修复。
4. 完成后，HTML 报告与 JSON 结果自动保存到当前会话「产物」，可以预览和下载。「查看报告」直接打开报告。报告使用平台已有文件存储，沙箱暂停或销毁后仍可访问。

仅打开面板、读取历史或轮询状态不会唤醒沙箱。点击运行会创建或恢复当前会话绑定的沙箱，执行准备只要求 shell 能力。运行期间保持沙箱活动；停止只结束该次验证的进程组，保留原有项目服务。同一会话一次执行一个任务，默认执行限时 300 秒，接口最大允许 900 秒。

「代码更新后自动验证」默认关闭，用户启用后仅在面板打开且检测到代码版本变化时触发。刷新页面或重新打开面板会恢复历史，不重新执行。

## 支持范围

| 项目 | 识别与结果 | 执行粒度 |
| --- | --- | --- |
| Node / TypeScript | 遵循 packageManager 和锁文件，支持 Node Test Runner、Vitest、Jest、Playwright 结构化结果 | 项目、套件、文件；Node/Vitest/Jest 提供可靠选择器时支持用例重跑 |
| Go | `go.mod`，`go test -json` 和 `go build` | 项目、包 |
| Python | `pyproject.toml`、`pytest.ini` 或 `setup.cfg`，pytest JUnit | 项目、文件 |
| 自定义命令 | 项目配置中的明确命令，保留退出码和日志 | 验证项 |

Node 在声明依赖且缺少 `node_modules` 时，用识别出的包管理器安装依赖；npm 有锁文件时使用 `npm ci`。Python/Go 需要沙箱中已有相应运行时，Python 还需要 pytest 及业务依赖。缺少依赖、运行时或业务配置会显示环境阻塞／执行错误，不会显示断言通过。Playwright 的浏览器运行时和业务服务需要项目自行准备。

用例从运行器真实报告获取，不读取源码正则猜测用例。0 个测试、全部跳过、无结构化结果、断言失败、环境阻塞、取消和超时分别显示。自定义测试命令只返回退出码 0 而没有用例证据时，结果为未知。

扫描最多 500 个目录、深度 6，跳过依赖、构建、隐藏目录和沙箱 input/output。达到扫描边界时展示识别不完整提示；测试文件目标最多 300 项。执行日志最多保留末尾 100,000 字符。

## 前后端联通与自定义验证

在项目清单所在目录增加 `.weknora/validation.json`：

```json
{
  "services": [
    { "name": "前端首页", "url": "http://127.0.0.1:5173/", "status": 200 },
    { "name": "后端健康检查", "url": "http://127.0.0.1:3000/health", "status": 200 },
    { "name": "前端代理接口", "url": "http://127.0.0.1:5173/api/health", "status": 200 }
  ],
  "checks": [
    { "name": "集成验证", "kind": "custom", "command": "npm run verify:integration" }
  ]
}
```

`services` 只读探测沙箱回环地址，检查预期 HTTP 状态；不会猜测并启动项目服务。平台保留端口不允许探测。`checks` 在该项目目录执行已配置的命令，最多 30 项，支持 tests/build/types/lint/custom 分类。

沙箱内部 HTTP 200 不能证明用户浏览器能打开预览。执行完成后的「检查浏览器预览入口」由当前浏览器请求预览专用健康路径，经过预览网关到沙箱；结果单独保存为浏览器访问证据。DNS／代理访问失败与沙箱服务不可达分开处理。它不验证页面 JS、业务交互或所有资源，完整页面交互需要项目的浏览器测试。

## 版本、AI 与存储

每次执行记录 Git HEAD、工作区指纹和各项目所属仓库。代码变化后标注历史结果；执行期间变化会标注版本不确定。Git 提交列表只显示同一仓库、干净提交且没有版本变化的完整项目验证摘要，文件／用例级运行不会为整个提交背书。

Agent 的 `workspace_tests` 工具支持 discover/run/status/history/cancel，与手动运行使用同一后端服务、run_id 和报告格式。AI 修复由用户发送回填提示后进行，结果仍以重新执行的真实报告为准。

运行记录保存于 `workspace_test_runs`，列表返回最近 30 次；报告复用 `message_artifacts` 和既有文件存储。会话所有权同时在接口和服务校验。报告发布采用事务，重复状态查询不会重复创建报告。

上线前需重启后端，应用 PostgreSQL `000115` 或 SQLite `000033` 迁移；按原流程构建前端。PostgreSQL `000115` 为已经执行过早期 `000114` 的数据库补齐运行控制字段，不重建表，也不删除历史报告。没有新增前端依赖或单独任务服务。

## 验证命令

```sh
python3 scripts/test_workspace_tests.py
go test ./internal/application/service ./internal/application/repository ./internal/handler ./internal/agent/tools ./internal/database ./internal/container -run 'TestWorkspaceTest|TestSQLiteMigrationsCreateVersionedSchema|TestMigrationDirectoriesLoad|TestWebPreview' -count=1
```

在 `frontend` 目录执行：

```sh
node --import tsx --test src/composables/useWorkspaceTests.test.ts src/utils/workspaceValidation.test.ts src/utils/workspaceEvents.test.ts src/components/workspace/ConversationWorkspace.test.mjs
npm run type-check
```
