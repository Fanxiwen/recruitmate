# 公司内部招聘系统（recruitmate）· 技术设计方案

> 版本 v2.0（Go 后端版）· 状态：已确认，进入开发
> 目标：前后端分离、AI 驱动的内部招聘平台。内部端：HR/部门负责人发布岗位与处理简历；外部端：求职者浏览岗位与投递简历；AI 对简历与岗位要求做智能匹配与排序。

---

## 1. 项目概述

招聘业务分两个世界：

- **内部世界（效率工具）**：HR 每天面对成百上千份简历，核心痛点是「如何在最短时间内从海量简历中定位合适的人」。内部端每个设计决策都服务于**筛选效率**与**决策可解释**。
- **外部世界（品牌窗口）**：求职者找岗位的体验，核心是「浏览流畅、投递门槛低、反馈可见」，同时是雇主品牌的一部分。

系统用 **AI 简历智能匹配** 把两个世界连起来：简历进入系统后自动解析、打分、排序，HR 打开岗位看到的是一份已经排好序、带匹配理由的候选人列表。

---

## 2. 角色与核心场景

### HR（招聘专员）— 最高频用户
| 场景 | 产品动作 |
|---|---|
| 发布岗位 | 填写岗位信息 → 草稿 → 提交审批 → 发布到外部端 |
| 每日处理新简历 | 打开「待处理」列表，**默认按 AI 匹配度排序**，顶部是最可能合适的人 |
| 快速决策 | 查看 AI 匹配理由/技能命中标签 → 通过 / 淘汰 / 转推其他岗位 |
| 批量处理 | 勾选多份简历批量淘汰、打标签、发通知 |
| 推进候选人 | 看板视图流转：待处理 → 初筛 → 面试 → Offer → 入职/关闭 |
| 岗位复盘 | 投递漏斗、来源渠道、招聘周期统计 |

### 部门负责人（Hiring Manager）
- 发起用人需求，审批本部门岗位发布；
- 查看本部门岗位候选人（**部门级数据隔离**），参与面试并评价。

### 求职者（Candidate）
- **无需登录即可浏览**岗位（降低流失），支持关键词搜索、部门/类型/地点筛选；
- 投递时填写基本信息 + 上传简历（支持粘贴文本兜底）；
- 用邮箱验证码登录查看投递状态；移动端友好（响应式）。

### 管理员（Admin）
- 内部账号管理、角色分配、系统配置（AI 权重、数据保留策略等）。

---

## 3. 关键产品决策

1. **外部端浏览不强制登录**，投递动作才要求身份。浏览门槛每降低一步，投递转化率就高一分。
2. **内部端围绕「筛选效率」设计**：默认匹配度排序、键盘快捷键（j/k 切换、Enter 通过、x 淘汰）、批量操作，单份简历决策目标 ≤ 30 秒。
3. **可解释 AI**：打分必须附带理由与技能命中/缺失标签。HR 信任系统的前提是能看懂系统为什么这么排。
4. **无 AI Key 也能跑**：未配置 AI 服务时自动降级为规则匹配（关键词/技能命中），保证系统可用性与演示性。
5. **一份简历可投多岗，匹配分按岗位独立计算并缓存**。
6. **求职者数据可被遗忘**：保留期自动匿名化 + 手动删除。

---

## 4. 功能范围与迭代计划

### M1 — 骨架与核心闭环（无 AI）
- Monorepo 工程 + CI + Docker 开发环境
- 内部端：登录（RBAC）、岗位 CRUD + 审批流、候选人列表
- 外部端：岗位列表/详情、搜索筛选、投递（简历上传）、投递状态查询

### M2 — AI 匹配引擎（核心）
- 简历解析（LLM 结构化抽取）
- JD/简历向量化 + 语义相似度（pgvector）
- LLM 评委评分 + 匹配理由（结构化输出）
- 匹配度排序列表 + 理由展示 + 硬性条件标记
- asynq 异步队列 + 失败重试 + JD 变更批量重算

### M3 — 效率与协作（后续）
- 看板 + 批量操作 + 智能标签；面试安排与评价；邮件通知；重复简历检测

### M4 — 洞察与完善（后续）
- 招聘数据看板；相似简历聚类（可选）；生产部署文档、监控、数据保留策略

---

## 5. 系统架构

```
                 ┌──────────────────────────────┐
 求职者 ──HTTPS─▶ │  careers（外部端 · React SPA） │──┐
                 └──────────────────────────────┘  │
                 ┌──────────────────────────────┐  │   REST API (/api/v1)
 HR/负责人 ──HTTPS▶│  web（内部端 · React SPA）      │──┼──────────▶ ┌────────────────────────┐
                 └──────────────────────────────┘  │              │  api（Go · 单一二进制）  │
                                                    │              │  ─ RBAC 认证            │
                                                    └─────────────▶│  ─ 业务逻辑             │
                                                                   │  ─ 文件上传             │
                                                                   └───┬─────────┬──────────┘
                                                       asynq 任务队列 ◀──┘         │
                                                       （Redis 驱动）               │ pgx/sqlc
                                                                    ┌─────────────▼──────────┐
                                                    Eino (AI 编排)  │ PostgreSQL + pgvector   │
                                                    ─ DeepSeek 评委  │ Redis（asynq + 验证码）  │
                                                    ─ SiliconFlow    │ MinIO（简历文件）        │
                                                      embedding      └────────────────────────┘
```

- **前后端分离**：两个 React SPA 通过同一套 REST API 交互；OpenAPI 规范由后端生成，前端据此生成类型安全的 API 客户端，杜绝接口漂移。
- **AI 处理与业务请求隔离**：简历解析/打分是耗时任务，走 asynq 异步队列；投递请求毫秒级返回，AI 结果就绪后内部端实时可见（轮询/SSE）。
- **部署形态**：Go 编译为单一静态二进制 + 多阶段 Docker 镜像（distroless），前端构建为静态资源由 Nginx/CDN 托管——正是 Go 栈的运维优势。

---

## 6. 技术选型与理由

| 层 | 选型 | 理由 |
|---|---|---|
| 后端语言 | Go 1.24 | 单二进制部署、内存占用低、并发模型适合高投递量；运维成本最低 |
| Web 框架 | Gin | 最主流、生态成熟，swaggo 文档与中间件开箱即用 |
| 数据库访问 | pgx + sqlc + goose | sqlc 从 SQL 生成类型安全代码（编译期检查 SQL）；goose 嵌入式迁移 |
| 数据库 | PostgreSQL 17 + pgvector | 语义检索与业务数据同库，免去单独向量库运维 |
| 任务队列 | asynq（Redis） | Go 原生任务队列：重试、去重、定时任务，AI 流水线天然适配 |
| AI 编排 | **Eino（cloudwego）** | Go 的 LLM 应用框架：OpenAI 兼容组件直连 DeepSeek/SiliconFlow，支持结构化输出与回调埋点 |
| 文件存储 | MinIO（S3 兼容） | 本地开发 MinIO，生产无缝换 OSS/S3 |
| 内部端 | React 18 + Vite + TS + Ant Design 5 + TanStack Query + Zustand | Ant Design 是中后台效率场景事实标准（表格/批量操作开箱即用） |
| 外部端 | React 18 + Vite + TS + Tailwind CSS | 轻量美观、响应式；M4 可选 SSR 加固 SEO |
| API 客户端 | openapi-typescript + openapi-fetch | 从后端 OpenAPI 自动生成类型安全客户端，前后端契约单一来源 |
| 测试 | Go：testify + httptest + testcontainers；前端：Vitest + Playwright E2E | 核心链路（发布→投递→打分→筛选）必须有 E2E 保障 |
| 工程 | pnpm workspaces（前端）+ go.work（可选）；Makefile；GitHub Actions CI | 单仓库多应用，一条命令开发/构建 |

---

## 7. 数据模型（核心表）

```
users(id, email, name, password_hash, role[admin|hr|hiring_manager], department_id, ...)
departments(id, name, ...)

job_postings(
  id, title, department_id, owner_id, approver_id,
  status[draft|pending|open|closed], headcount,
  salary_min, salary_max, location, job_type,
  description,                -- 岗位职责（富文本）
  requirements jsonb,         -- 结构化要求：必备技能[]/加分技能[]/学历/年限
  published_at, closed_at
)

candidates(id, email, phone, name, ...)          -- 外部求职者
applications(
  id, candidate_id, job_id, stage, status, source,
  resume_file_key, resume_text,                  -- 原文与原件
  parsed_resume jsonb,                           -- AI 结构化抽取结果
  match_score numeric, match_detail jsonb,       -- 打分与理由（可解释）
  hard_pass bool, submitted_at, ...
)
interviews(id, application_id, scheduled_at, interviewer_ids, feedback)
comments / tags / notifications / audit_logs
```

设计要点：
- **requirements 结构化**（不是自由文本）：硬性条件（学历/年限/必备技能）可被程序校验并解释，这是「可解释匹配」的基础。
- **match_score/match_detail 落在 application 上**：一份简历投 N 个岗位有 N 个独立评分；JD 修改 → 只对该岗位投递重算。
- **audit_logs 全链路审计**：谁看了哪份简历、谁改了状态，都有记录。

---

## 8. AI 简历匹配引擎（核心设计）

### 8.1 流水线（asynq 异步任务）

```
简历上传 ──▶ ① 文本提取（PDF: pdfcpu / DOCX: 标准库 zip+xml）
         ──▶ ② LLM 结构化解析（Eino 结构化输出）→ parsed_resume JSON
         ──▶ ③ 硬性条件检查（纯规则，可解释）
         ──▶ ④ Embedding 语义相似度（SiliconFlow bge-m3，pgvector 余弦距离）
         ──▶ ⑤ LLM 评委打分 + 理由（DeepSeek，JSON 输出）
         ──▶ ⑥ 综合分落库 → HR 端排序列表
```

每步独立 asynq 任务 + 指数退避重试（≤3 次），失败标记「解析失败」供人工兜底；asynq 唯一任务保证同一投递不重复打分。

### 8.2 多因子评分模型（0–100，权重可配置）

```
综合分 = 0.45·LLM评委分 + 0.30·语义相似度 + 0.25·结构化规则分

硬性条件（学历/年限/必备技能）任一不满足 → hard_pass 标记，
列表显示「不满足硬性要求」并折叠到过滤区，仍可见、可恢复。
```

- **结构化规则分**：必备技能命中率、年限达标度等纯规则计算，稳定可解释。
- **语义相似度**：JD 与简历 embedding 余弦相似度，捕捉「表述不同但能力相同」。
- **LLM 评委**：按固定 rubric 输出 JSON：`{score, strengths[≤3], gaps[≤3], risk, summary}`，是 HR 决策时读的「人话」。

### 8.3 成本与性能控制（漏斗式）

1. 规则检查 + embedding（便宜）先算，覆盖全部投递；
2. 仅对**排名前 N**（默认 Top 50）调用 LLM 评委精排；
3. 结果全量缓存：同一简历投同一岗位不重复算；JD 修改 → 该岗位批量重算（asynq 定时任务，低峰执行）。

### 8.4 降级与可观测

- 无 AI Key → 纯规则匹配模式，系统照常工作；
- 记录每次评分的模型、版本、耗时与 token 成本（Eino callback 埋点），支持后续 A/B 调权；
- 结构化日志（slog）贯穿流水线，每份简历可追溯处理链路。

---

## 9. 大量简历场景的配套设计

| 能力 | 说明 |
|---|---|
| 匹配度排序 + 可解释理由 | 默认视图，决策依据一眼可见 |
| 硬性条件过滤 | 不达硬标的折叠到「不满足」区，不污染主列表 |
| 看板 + 批量操作 | 勾选批量淘汰/打标签/发通知；键盘快捷键 |
| 智能标签 | 「资深后端」「应届」等自动打标 |
| 重复检测 | 同邮箱/手机号多投 → 合并展示 |
| 保存筛选器 | HR 常用筛选存为视图（如「A 岗 · 3 年+ · 已评分」） |
| 新简历通知 | 邮件/站内提醒，高匹配简历优先置顶 |
| 岗位内搜索 | 全文检索 parsed_resume 技能关键词 |

---

## 10. API 设计

```
/api/v1/public/  — 外部端（无需认证）
  GET  /jobs                     岗位列表（筛选/分页）
  GET  /jobs/:id                 岗位详情
  POST /jobs/:id/applications    投递（multipart 简历 + 基本信息）
  GET  /my/applications          投递状态（邮箱验证码 token）

/api/v1/internal/ — 内部端（JWT + RBAC）
  POST   /jobs                     发布岗位
  GET    /jobs/:id/applications    候选人列表（默认按 match_score 排序）
  GET    /applications/:id         简历详情（含 AI 理由；部门数据隔离）
  PATCH  /applications/:id/stage   流转状态
  POST   /applications/batch       批量操作
  POST   /jobs/:id/approve         审批发布（hiring_manager）
  GET    /jobs/:id/stats           投递漏斗统计

后端 swaggo 注解 → swagger.json → openapi-typescript 生成前端客户端类型。
```

---

## 11. 安全与合规

- **认证**：内部端 JWT（golang-jwt）+ Argon2id 密码哈希 + RBAC；外部端求职者用邮箱验证码（Redis 存储、限流）。
- **数据隔离**：hiring_manager 只能访问本部门岗位与候选人，服务端强制校验（中间件级，不依赖前端）。
- **文件安全**：上传白名单（pdf/docx/txt）+ 大小限制；生产加病毒扫描。
- **隐私**：手机号/邮箱默认脱敏；简历保留策略（180 天自动匿名化）+ 手动删除；审计日志。

---

## 12. 仓库结构与工程规范

```
recruitmate/
├── apps/
│   ├── api/                  # Go 后端（独立 go.mod）
│   │   ├── cmd/server/       # 入口
│   │   ├── internal/         # config/server/auth/domain/repo/service/handler/worker/ai/file
│   │   └── migrations/       # goose 迁移
│   ├── web/                  # 内部端（React + AntD）
│   └── careers/              # 外部端（React + Tailwind）
├── packages/
│   ├── shared-types/         # 共享 TS 类型
│   └── api-client/           # OpenAPI 生成的类型安全客户端
├── docker-compose.yml        # postgres+pgvector / redis / minio / api
├── Makefile                  # 统一开发/构建命令
├── docs/
└── .github/workflows/ci.yml  # golangci-lint + go test + pnpm build + 镜像构建
```

- Conventional Commits；功能分支 + PR 评审；每个里程碑可独立运行；
- README：架构图、产品截图、快速启动（`docker compose up` + seed 演示数据）、演示账号。

---

## 13. 已确认决策记录

| 项 | 决策 |
|---|---|
| 技术栈 | 前端 React 全家桶；后端 Go（Gin + sqlc + pgx）；AI 编排用 Eino |
| AI 供应商 | LLM：DeepSeek（OpenAI 兼容，Eino openai 组件接入）；Embedding：SiliconFlow bge-m3 |
| GitHub | 仓库名 `recruitmate`；本机安装 gh CLI 登录后创建 |
| 本轮范围 | M1（核心闭环）+ M2（AI 匹配引擎） |
