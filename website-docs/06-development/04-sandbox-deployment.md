# 沙箱部署与排障

技能使用流程见[技能目录与沙箱](../03-features/22-skills-sandbox.md)，字段定义见[沙箱与技能 API](../04-api/02-api-sandbox-skills.md)。本文补充部署、模板、终端、网页预览和后端接入的运维要求。

## 后端与模板

空间命名配置可选择 Docker、CubeSandbox、E2B，本文说明这三类后端的部署。Lite 桌面客户端另有依赖操作系统隔离能力的 `host` 后端，不属于服务端命名配置，构建要求见下文 [Lite 本机沙箱](#lite-本机沙箱)；旧 `local` 宿主机进程后端已移除。Docker 直接使用 Engine API；E2B 使用控制面 REST 与 envd 数据面；Cube 保留专用适配器处理模板和网络策略。

标准镜像由 `docker/Dockerfile.sandbox` 定义，包含 Python 3.12、Node.js 20、Bash、jq 与 `/workspace`。命令和文件操作默认使用沙箱内 root；保留 UID 1000 的 `user` 供显式按账号执行。跨会话隔离由容器或远端沙箱提供，不能把工作目录约定解释为 root 的文件权限限制。

| 构建 target | 镜像标签后缀 | 用途 |
| --- | --- | --- |
| `sandbox` | 无 | Docker 会话镜像；E2B CLI 模板基础镜像 |
| `cube` | `-cube` | Cube CLI 模板，含 envd |

镜像名为 `wechatopenai/weknora-sandbox:<版本><后缀>`。模板 ID 属于具体集群或账号，不应把其他部署的 ID 直接复制过来。生产模板应对应已验证的应用版本；构建 target 和发布架构以仓库 Dockerfile、发布工作流为准。

## Docker 部署

Docker 后端默认关闭。系统管理员在「系统设置 → 网络安全」启用；尚未落库时可由 `WEKNORA_SANDBOX_DOCKER_ENABLED=true` 回退。关闭后仍可查看和删除已有配置，但不会新建会话容器。

一个配置连接一个 daemon，一个会话使用一个长驻容器。没有跨主机调度；不同 app 副本若连接各自独立的本机 daemon，不能假定会共享容器和技能快照。

| 配置 | 默认值或要求 |
| --- | --- |
| daemon 地址 | 留空时检测 `DOCKER_HOST` 或当前 Docker context；远端使用 `tcp://host:2376` |
| TLS 证书目录 | 远端 TCP 必填，app 可读的目录中包含 `ca.pem`、`cert.pem`、`key.pem` |
| CPU / 内存 / PID | 默认 2 核 / 2 GiB / 512 进程 |
| 空闲 TTL | 默认 1800 秒 |
| 网络模式 | 仅 `bridge` 或 `none`；不接受 `host`、`container:` 或自定义网络名 |
| OCI runtime | 可选择 daemon 已安装的 runtime，如 `runsc` |

app 运行在容器里时，要挂载实际 Docker socket，或改连远端 daemon。socket 授予控制宿主机 Docker 的能力，只应交给可信 app。入口脚本在降权前根据 socket GID 配置 appuser 的组；不要通过 `chmod 666` 放开 socket。若 socket 为 `root:root` 且仅所有者可写，应先在宿主机配置合适的非 root 组权限。

优先使用标准镜像。自定义镜像需满足适配器的 root、Bash、GNU `find -printf`、coreutils `timeout` 和可写工作区要求；会话文件检查点与回退还依赖 Git。HTTP 请求取消本身不会终止容器进程，适配器通过容器内 `timeout` 执行命令超时控制。

空闲清扫由 Create/Connect 触发并在后台运行，按容器创建时记录的 TTL 判断；不是 daemon 自带的定时器。当前没有硬寿命上限，能执行命令的脚本可以持续更新活跃标记，部署方需单独监控长期存活容器。

### 技能快照与磁盘

技能安装通过 `docker commit` 生成 `weknora-skill/` 本地镜像，后续会话从安装快照启动。它保存文件系统，不保存内存状态；当前不支持租户共享卷或 Docker 图形桌面。

增量快照继承旧镜像层，删除旧 tag 不一定释放磁盘；卸载技能会新增删除标记层，原文件仍可能留在父层。当前流程不自动压平镜像或跨 daemon 分发。监控层数和磁盘，清理前确认会话与快照引用；需要新底模时新建配置并重新安装技能。

## CubeSandbox / E2B 接入

1. 先准备可用的控制面、数据面网关和沙箱域名。在空间设置中填写 API、Proxy、domain 与凭据；私网/回环端点需要显式允许私网地址，仍不允许云元数据地址。
2. 点击「连接并继续」，验证控制面后加载模板目录。连接通过只证明控制面可用。
3. 缺少标准模板时显式创建，等待状态为 `READY` 再选择。Cube 应使用带 envd 的 `-cube` 变体；普通 Docker 镜像不提供 `:49983/health`。
4. 执行「完整验证」，实际创建沙箱、执行探针并销毁，确认数据面和脚本环境也可用。
5. 保存后在智能体选择该配置，并验证附件读取、命令状态保持和产物下载。

`proxy_url` 用于自托管数据面网关：连接网关时保留沙箱 Host 路由，适合没有泛域名 DNS 的环境。控制面可访问而执行失败时，重点核对 Proxy、sandbox domain、入站凭据和 app 到网关的可达性。

Cube guest DNS 属于模板配置。更改 DNS/镜像后需重建模板才会进入新环境。已经安装技能的配置不能更换或重建底模，应新建配置并安装技能，避免让快照与底模不一致。

### Cube 拉取镜像超时

若构建错误包含 `Get "https://index.docker.io/v2/": dial tcp ...:443: i/o timeout`，失败发生在 **Cube 构建节点访问镜像仓库** 时，尚未启动沙箱。先处理构建节点的出站网络、DNS 或代理。WeKnora 的 `proxy_url` 是沙箱数据面网关，不是镜像拉取的出站代理，修改 guest DNS 无法修复这一阶段的网络。

也可以将 `wechatopenai/weknora-sandbox:main-cube` 的 `linux/amd64` 镜像同步到 Cube 可访问的镜像仓库，在集群构建模板后，在 WeKnora 中选择对应模板 ID。自行构建时使用 `docker/Dockerfile.sandbox` 的 `cube` target；私有仓库认证应配置在实际执行拉取的 Cube 构建环境中。

多副本必须配置共享 Redis，保存 session→sandbox 绑定和访问状态。仅单实例开发可使用内存存储。已有沙箱不会自动应用后端配置变更；技能更新的 `next_turn` / `new_session` 策略见[技能目录与沙箱](../03-features/22-skills-sandbox.md)。

| 现象 | 排查方向 |
| --- | --- |
| 连接验证失败 | API 是否误填 Dashboard 地址；凭据、TLS、私网开关是否正确 |
| 连接成功但执行失败 | Proxy、sandbox domain、网关路由与入站 token |
| 模板构建失败 | 查看集群返回的构建错误；镜像是否能拉取、架构是否匹配、Cube 镜像是否带 envd |
| 沙箱无法出网 | 模板网络策略、guest DNS、集群出站代理；允许私网控制面不等于允许脚本出网 |
| 安装依赖失败 | 默认拒绝出站时是否放行软件源；技能安装与会话使用同一网络策略 |
| 重连后状态丢失 | Redis 是否共享、TTL 是否到期、技能更新是否触发重建 |

## 会话代码保留与恢复

Cube/E2B 的新会话沙箱使用 `onTimeout=pause`、`autoResume=true`：TTL 到期后暂停，后续对话连接同一个沙箱时恢复。此前 Cube 使用 `kill`，会随 TTL 到期删除工作区。此修改需要部署新的 WeKnora 后端，只影响之后创建的实例；不要为了切换策略直接删除仍有代码的旧实例，应先下载备份。Docker 仍按空闲 TTL 清理容器。

暂停不能代替独立备份。远端会话每轮结束时（包括取消和失败），在现有沙箱内打包 `/workspace` 的普通文件为输出目录中的 `workspace-source.zip`，随后由产物收集器保存到 WeKnora 的文件存储并关联该轮消息。成功上传后的备份不依赖原沙箱，压缩包带有轮次标识，确保回到旧代码时仍记录当前恢复点；历史版本会占用文件存储空间。备份不包含根目录的 `input` / `output`、配置的产物目录、Git 元数据、符号链接，以及 `node_modules`、虚拟环境和常见缓存；解压后需要重新安装依赖。宿主机工作区不会自动打包。

备份最多扫描 20000 个目录、20000 个文件和 200 MiB 源文件，压缩后不得超过产物上传上限（默认 50 MiB），打包限时 30 秒。需要沙箱提供 Python 3。超限、读取失败或上传失败均不会中断回复，也不会生成部分源码备份；可在后端日志查找 `WorkspaceBackup` / `ArtifactCollector`。请以会话产物中实际出现的文件为保存成功的依据。轮次完成前的进程崩溃仍可能丢失尚未备份的修改。

沙箱被删除或重建后，到「产物」下载最近一次成功保存的 `workspace-source.zip`，重新作为附件上传，让 Agent 解压到新沙箱的 `/workspace` 后继续工作。目前不会自动把备份还原到新实例。备份的保留随消息、产物和文件存储的生命周期；部署时应持久化文件存储，并使用持久化 Redis 保存沙箱绑定及入站凭据，避免 WeKnora 重启后失去现有实例的访问凭据。已销毁且从未备份的文件无法从空白新沙箱恢复。

## 网页与单文件预览

右侧「代码」使用文件树和文件工具栏。选中 HTML、SVG 或 Markdown 后，可在「代码 / 预览」间切换，支持复制、刷新和手机宽度。单文件预览在隔离 iframe 内运行，不依赖图形桌面，也不必启动 HTTP 服务；HTML 可加载同工作区中的 CSS、普通 JavaScript 和 SVG 图片。远程资源、网络请求、二进制图片及需要打包的模块依赖不属于此模式，资源加载失败会显示提示。

右侧「预览」可直接嵌入已经启动的项目 URL，基础 CLI 沙箱即可承载项目，无需 Chromium、VNC 或桌面模板。部署步骤：

1. 在沙箱中安装项目依赖并启动 HTTP 服务；容器内的服务监听 `0.0.0.0`，保持进程运行。
2. 通过部署环境已有的端口映射或预览网关，生成用户浏览器能访问的 URL。此入口不自动创建映射、不代理任意 URL，也不向 iframe 注入 WeKnora 登录凭据或沙箱 API Key。
3. 将 URL 粘贴到地址栏。支持刷新、相对路径导航、桌面/手机宽度、全屏和新窗口打开。

`localhost` 指用户当前电脑，不能用来直接访问远端沙箱内部的回环地址。项目应使用与 WeKnora 不同的来源（独立域名或端口），避免项目脚本访问应用来源；不要把预览页面重定向到 WeKnora 来源。WeKnora 使用 HTTPS 时，远程项目也需要 HTTPS。服务的 `X-Frame-Options` 和 CSP `frame-ancestors` 必须允许嵌入；涉及第三方 Cookie 的登录页面也可能受浏览器限制。跨来源 iframe 无法可靠判断页面是否被拦截，可使用面板底部排障提示或新窗口打开来验证实际地址。

预览面板仅提供网页预览，不再显示远程桌面标签，也不查询桌面能力或建立桌面连接。基础 CLI 沙箱即可使用网页预览。

## 交互终端

对话侧栏的终端通过 WebSocket 连接会话沙箱，支持 Cube/E2B 和 Docker。浏览器不能在 WebSocket 握手里携带认证头，因此先用已登录的 POST 换取短期票据，再把票据放在握手 query 中：

| 能力 | 取票据 | WebSocket |
| --- | --- | --- |
| 终端 | `POST /api/v1/sessions/:session_id/sandbox/terminal-ticket` | `GET /api/v1/sessions/:id/sandbox/terminal?ticket=...` |

入口代理必须为终端路径转发 WebSocket Upgrade、放宽读超时，并避免在访问日志中记录 ticket query。标准 frontend Nginx 已为 `^/api/v1/sessions/[^/]+/sandbox/terminal$` 配置不含 query 的日志格式；自定义 Ingress 需自行处理。终端连上后，服务端会复核登录状态、空间成员和会话归属。接口参数见[终端 API](../04-api/02-api-sandbox-skills.md#会话交互终端)。

终端在无操作时按 `terminal_idle_disconnect_sec` 断开（默认 900 秒，范围 60 秒到 24 小时），键盘输入和 PTY 输出均计为活动；之后由提供商 TTL 决定沙箱暂停。

远程桌面的票据、WebSocket 中继和模板构建功能已移除。网页预览使用基础执行环境，不依赖 VNC、XFCE 或 websockify。

Docker 终端是一个 `TTY=true` 的 exec，Engine API 不支持重新附着，因此行为与 Cube/E2B 不同：

- 终端打开期间会定期刷新空闲回收用的活动标记，容器不会因 `DockerIdleTTL` 被回收；关闭后按 `DockerIdleTTL` 正常回收。
- 终端断开（关闭面板、空闲断开、网络中断）时，WeKnora 会像关闭终端窗口一样向该 Shell 及其子进程发送 SIGHUP，前台任务随之结束，`nohup` 启动的后台任务保留。
- 前端不会对 Docker 终端自动重连，用户点「重新连接」会得到一个新 Shell。
- 沙箱未运行时（容器已停止或已被空闲回收）提示「可能已被回收」，启动后若容器已被删除会新建沙箱，之前的文件不会保留。


## Lite 本机沙箱

`host` 后端只编译进带 `desktop` 构建标签的 Lite 桌面程序（`cmd/desktop/wails.json` 的 `build:tags`），服务端和单二进制 Lite 不包含。它在未选沙箱配置的会话中运行命令，用法见[技能目录与沙箱](../03-features/22-skills-sandbox.md#lite-host)。

| 平台 | 状态 |
| --- | --- |
| macOS | 使用系统 `sandbox-exec`（Seatbelt）执行每条命令；启动时检测不可用则不启用 |
| Windows | 尚未实现，报告不可用，不会退化为无隔离执行 |
| Linux | 不支持 |

每条命令都是新的本机进程，没有会话级实例，因此不写入会话的沙箱绑定，也不做工作区检查点。用户通过系统目录选择框批准的项目目录保存在 `desktop-prefs.json` 的 `project_dirs`；会话只能绑定列表中的目录本身，不能绑定其子目录或手工输入的路径。`approval_mode` 当前只支持 `auto`（在工作区内自由读写、禁止联网），写入其他值会被拒绝。偏好文件位置见[桌面客户端](../05-clients/05-desktop.md#_5-偏好设置存储-cmd-desktop-prefs-go)。

## 开发验证

单元测试不需要真实 daemon：

```bash
go test ./internal/sandbox -run 'TestDocker' -count=1
```

真实 Docker 验证需要先构建标准镜像，会创建测试容器：

```bash
docker build -f docker/Dockerfile.sandbox --target sandbox -t wechatopenai/weknora-sandbox:dev .
DOCKER_INTEGRATION_IMAGE=wechatopenai/weknora-sandbox:dev \
go test -tags=docker_integration ./internal/sandbox \
  -run '^TestDocker.*Integration' -count=1 -v -timeout=15m
```

远端接入的一致性测试位于 `internal/sandbox/cube_integration_test.go` 与 `e2b_compatible_integration_test.go`，运行前按文件开头配置测试集群凭据。验证范围应包含会话状态保持、Shell 复用、附件暂存、产物收集和超时，而不能只测 Health。测试结束后确认测试实例已清理。
