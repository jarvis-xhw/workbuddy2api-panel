# Fork 集成说明（2026-10-20）

本文件记录本 fork 在**上游最后提交 `6b31db6`（2026-10-12）**之上的集成结果：
吸收了上游 30 个 open PR 中的 25 个，另加 1 个由 issue 结论直接落地的补丁。

- 上游：`linguo2625469/workbuddy2api-panel`
- 本 fork：`shinny-xionghongwei/workbuddy2api-panel`
- 集成分支：`integration/2026-10-20`（已 fast-forward 到 `main`）
- 实验分支：`feature/multi-pool-183`（Trae / ZCode / AutoClaw 多池大改，见下文）
- 验证：`go build ./...` + `go test ./...` 全绿（含 `internal/panel` 前端 node 冒烟测试）

---

## 1. 已合并的 PR（25）

| # | 标题 | 落点 |
|---|---|---|
| 233 | 国际版 11-128 通道校验分流 + 始终发送 X-Conversation-ID | `server/handler.go`、`upstream/headers.go` |
| 232 | 裸模型名跨域回退开关 `pool.cross_domain_fallback`（issue #135） | `pool`、`server` |
| 230 | Dockerfile 改用国内 Go 模块源 | `Dockerfile` |
| 225 | `models.hidden` 模型黑名单（列表过滤 + chat 拦截） | `livecfg`、`server` |
| 223 | 出站代理支持环境变量（`HTTP(S)_PROXY`） | `upstream/transport.go` |
| 221 | 区分「模型无权限」与「无可用账号」，展示 Global 模型可用性 | `panel`、`pool`、`server` |
| 220 | 新增 `cn.chat_base`，修海外出口下 CN 模型目录全缺 | `cmd/server`、`server` |
| 217 | 同账号 refresh 单飞，避免 RT 复用撞 12153 掉登录（#215） | `upstream/client.go` |
| 213 | 接入联网搜索端点 `POST /v1/search/messages` | `server`、`upstream` |
| 212 | 到期提醒快照与「检查」同口径，按自然日聚合批次 | `pool`、`upstream`、`panel` |
| 205 | 任务补领清扫：周期幂等补领「已达标未领」（#174） | `panel`、`scheduler` |
| 204 | 一键完成遇「已达标未领」先补领再跳过（#174） | `panel/autotask.go` |
| 203 | 模型级 6004 双时钟，倒计时跟随 `until`（#171） | `panel/app.js` |
| 202 | 软冷却到期后不再透出过期 reason（#171） | `pool/state.go` |
| 199 | 缓存告警假阳性：miss 显式存在时对称反推 hit（#170） | `server/logging.go` |
| 196 | 补偿状态可观测化：先查后领 + 截止倒计时 + 面板入口（#189） | `scheduler`、`panel` |
| 193 | 静态 API Key 账号（无成长体系 / 企业口径额度） | `auth`、`panel`、`scheduler` |
| 185 | 6021 `queue.full` 归为渠道级忙：不罚号、不轮转、503 透传 | `server`、`upstream` |
| 178 | 账号备注 + 积分到期提醒折叠 | `panel` |
| 162 | 企业切换会话（`token_source=enterprise_switch`）门控 | `auth`、`scheduler`、`panel` |
| 159 | 剥离 content 里的 `reasoning_text` 片段，修 11101 | `upstream/payload.go` |
| 158 | 按地区独立配置轮询 / 顺序耗尽调度 | `pool`、`cmd/server` |
| 144 | 出站 DNS 可指定（`dns_servers` / `WB2A_DNS`） | `upstream/dns.go` |
| 143 | 手机端适配（抽屉侧栏 / 宽表卡片化 / 窄屏时序图） | `panel` |
| 138 | 「积分历史」：逐账号记录余额变动 | `credithist`、`panel` |

## 2. 未合并的 PR（5）与原因

| # | 原因 |
|---|---|
| 190 | 与 #233 **完全重复**：唯一改动（缺失时补 `X-Conversation-ID`）已被 #233 原样包含，合并只会产生冲突。 |
| 180 | 与 #185 **同源**（6021 队列满）。#180 只做到「不罚号但仍轮转换号」，换号仍会把无效请求放大 `MaxRotate` 倍；#185 在轮转循环内 fail-fast，语义更彻底，故取 #185。 |
| 198 | 与 #233 **语义冲突**：它把 11140 `request illegal` 一律降级为软冷却。#233 的结论是「11-128 是通道校验失败（不罚号）/ 11140 是账号级限制（硬禁用）」，粒度更细，故取 #233。 |
| 183 | 17.5k 行、79 文件的多池大改（Trae CN / ZCode / AutoClaw），引入 `modernc.org/sqlite` 等新依赖。**已单独合入 `feature/multi-pool-183` 分支并通过全量测试**，但不进 `main`：它与已合并的 #212/#220/#232/#233 在路由与配置装配处有 6 处冲突，且属于「引入第二套上游体系」的架构级变更，建议先独立验证再决定是否并线。 |
| 104 | 已被上游自身标为 `CONFLICTING`（37 文件、+4065/−670），且与已合并的 #204/#205（任务补领）/ #212（到期快照）/ #196（补偿）在计费与任务结果口径上重叠，需作者 rebase 后再评估。 |

## 3. Issue 处理情况

已被本次合并覆盖：

- **#170** 缓存命中率告警假阳性 → #199
- **#171** 面板「可用 + 历史 429」并排、6004 双时钟 → #202 / #203
- **#174** 任务「已达标未领」 → #204 / #205
- **#135** 裸模型名跨域选号 → #232
- **#215** RT 复用撞 12153 → #217
- **#187 / #188** 国际版缺 `X-Conversation-ID` 报 11-128 → #233
- **#189** 补偿状态不可见 → #196
- **#128** 图表 tooltip（含手机端自绘图） → #143
- **#231** 国际版「账号全被禁用但网页还能聊」→ #233（11-128 不再罚号、始终发 `X-Conversation-ID`）
  + 既有的 `ensureConsoleSystem`（global 域首条非 system 自动补 system，本次补 5 条回归测试）
- **#81** 历史 tool_call id 重复 → 见第 4 节自研补丁
- **#145 / #102 / #125 / #130 / #131 / #121** 等在上游快照前已合并

仍然开放、本 fork 未擅自实现（需要上游口径或架构决策）：

- **#84** PostgreSQL 存储（当前 Upstash/Redis + 本地快照）
- **#89** 原生 Responses API（#109 已 closed，未合入）
- **#41** API Key 分发给下游（社区明确反对，上游亦表态不做）
- **#43** global 域 `/v3/config` 按 UA 下发不同模型目录
- **#20** ZCode 反代 → 见 `feature/multi-pool-183`
- **#28** 会话终止不停止在途任务
- **#26 / #15** 国际版模型列表缺失（#221 只改善了可用性展示，目录本身仍依赖探测）

## 4. 额外补丁：出站 `tool_call.id` 唯一化（issue #81）

issue #81 的现场结论：客户端把 `tool_call.id` 截断到 40 字符，同一批并行调用被截成
同一个 id，上游按 id 建映射后判 `tool calls and tool results do not match`
（HTTP 400 / code 11148），并让该会话之后每轮都失败。

本 fork 在 `PrepareBodyOpt...` 管线里、`merge/repack/cleanup` 三步之后加了一步
`dedupeToolCallIDs`：

- 第 1 次出现的 id 保留原值，第 k 次（k≥2）改名为 `<id>-k`（若已占用则继续递增，避免撞车）；
- 对应的 `tool` 结果按顺序消费同一组新 id，保证配对不断；
- 健康请求零改动（幂等），命中时打一行 `[payload] dedupe_tool_ids: repaired N ...` 日志。

测试：`internal/upstream/payload_dedup_test.go`（截断并行批 / 健康请求 / 改名避碰 / 空 messages）。

## 5. 冲突解决记录（关键语义决策）

1. **#158 × #232/#220/#144**：`main.go` 启动装配与热重载两处冲突，均为「两边各加一行
   setter」，并集保留（`SetCrossDomainFallback` + `SetScheduling`）。
2. **#178 × #158/#221**：`frontend_test.go` 同一位置各追加测试函数，取我方全套测试 +
   手工追加 #143 的 `TestMobileJSChartTooltipInsideBar` 与语法检查改动（不做整文件覆盖）。
3. **#212 × #138/#205**：`client.go` 余额快照路径同时需要
   `notifyCredits`（积分历史）与 `buckets`（到期批次），合并为「企业口径分支内先 notify 再拼 bucket」；
   `scheduler.go` 的 `sweepInterval`（#205）与 `lastBalanceRefresh`（#212）两个 atomic 字段并存。
4. **#193 × #162**：`SupportsPersonalGrowth()` 只覆盖企业版与 API Key，**不覆盖**
   `enterprise_switch` 会话，因此门控统一写成
   `!a.SupportsPersonalGrowth() || a.IsEnterpriseSession()`，并在拒绝时按类型给不同提示
   （企业切换会话 → 重登指引；API Key → 无成长体系）。
5. **#193 × #212**：`UserResourceSnapshot` 的企业分流改用 `a.UsesEnterpriseBilling()`
   （企业版 + API Key），同时保留 #212 的按日聚合与我们新增的余额观察者回调。
6. **#183（实验分支）**：`ServeHTTP` 被两边同时改写，重写为「先 zcode/trae/autoclaw 分流，
   再做请求日志记账」；`app.js` 修掉重复 `const [d, pr]` 与一处丢失的 `};` 闭合。

## 5.5 2026-10-21 上游动态（重要）

上游在 2026-10-21 凌晨合并了 25 个社区 PR（与本 fork 吸收的集合基本一致），随后：

1. **发布停止维护声明**（commit `73a7560e`）：上游禁反代收紧、个人精力有限、非法转卖泛滥；
   仓库保持可用但不再更新、不再处理 issue / PR。**issue 与 open PR 均已清零**。
2. **修复了 PR #233 的一个致命笔误**（commit `d809396d`）：`11-128` 的业务码字面量
   被写成 `11-128`（少一个连字符，字节 `3131313238`），实现与测试同错所以测试全绿，
   但线上永远命中不了 → 11-128 落到 `ErrClient`，通道拒绝分流形同虚设。
   已同步进本 fork，并保留上游新增的字节级回归用例 `TestClassifyRealChannelRefusedBody`。

本 fork 已合并上游 `main`（同步提交 `dc6486c`），在此基础上继续维护。

## 6. 后续同步建议

- 上游若继续更新，先 `git fetch upstream && git rebase upstream/main`，冲突面集中在
  `cmd/server/main.go`、`internal/server/handler.go`、`internal/panel/app.js` 三处。
- 想启用多池：`git checkout feature/multi-pool-183`，重点回归 `/v1/models` 的
  `owned_by` 多池标记、`/trae|/zcode|/autoclaw` 反代路径与 `modernc.org/sqlite` 的构建产物体积。
- #104 重新评估的前提：作者 rebase 后与 #204/#205/#196 的任务结果口径对齐。
