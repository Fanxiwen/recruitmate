# recruitmate · 公司内部招聘系统

AI 驱动的内部招聘平台：HR/部门负责人在内部端发布岗位、高效筛选简历；求职者在外部端浏览岗位、投递简历；AI 引擎自动解析简历、评估匹配度并为每位候选人给出**可解释的排序与理由**。

> 外部端已按「中国-葡语（西语）国家经济贸易服务中心（中葡经贸中心）」品牌化：葡萄牙绿 + 浑天仪金配色、瓷片纹样装饰、机构名称与葡语品牌语。品牌文案集中在 `apps/careers/src/components/{Header,Footer}.tsx` 与 `apps/careers/src/pages/HomePage.tsx`，可自由调整。

前后端分离 · Go + React · Monorepo

## 功能总览

| 端 | 能力 |
|---|---|
| 内部端（HR / 部门负责人） | 岗位发布与审批流（草稿→审批→招聘中→关闭）、结构化岗位要求（必备/加分技能、学历、年限）、候选人列表按 AI 匹配度排序、匹配理由与技能命中/缺失可视化、硬性条件标记与过滤、看板阶段流转、批量操作、键盘快捷键、投递漏斗统计、部门级数据隔离 |
| 外部端（求职者） | 无需登录浏览岗位、搜索与筛选、上传简历或粘贴文本投递、邮箱验证码登录查看投递进度、移动端适配 |
| AI 引擎 | 简历结构化解析（DeepSeek）、语义相似度（SiliconFlow bge-m3 + pgvector）、LLM 评委打分与理由生成、多因子综合评分、异步队列与失败重试、**无 API Key 自动降级为规则匹配** |

## 架构

```
┌──────────────┐   ┌──────────────┐
│ apps/web     │   │ apps/careers │   React 18 + Vite + TypeScript
│ 内部端 React  │   │ 外部端 React  │   （共享 packages/shared-types 契约）
└──────┬───────┘   └──────┬───────┘
       │  REST /api/v1     │
       └────────┬──────────┘
        ┌───────▼──────────────────────────┐
        │ apps/api · Go (Gin + pgx)        │
        │  RBAC · 审批流 · 投递 · 审计      │
        │  asynq 任务队列（Redis）          │
        │  Eino 编排 AI（DeepSeek/SiliconFlow）│
        └───┬──────────┬──────────┬────────┘
   PostgreSQL 17   Redis 7     MinIO
   + pgvector     (队列/验证码)  (简历文件)
```

详细设计见 [docs/technical-design.md](docs/technical-design.md)。

## 快速开始

### 前置要求

- Go ≥ 1.27、Node ≥ 20 + pnpm ≥ 11
- PostgreSQL 17 + pgvector、Redis、MinIO（两种方式任选其一）

### 1. 启动基础设施

有 Docker 的机器：

```bash
docker compose up -d postgres redis minio
```

macOS 无 Docker（brew 原生服务；若数据库未初始化见下方「首次初始化」）：

```bash
brew install postgresql@17 pgvector redis minio
make infra-up   # brew services start postgresql@17 redis minio
```

<details>
<summary>首次初始化（仅 brew 方式第一次需要）</summary>

```bash
export PATH="/opt/homebrew/opt/postgresql@17/bin:$PATH"
psql -d postgres -c "CREATE ROLE recruitmate LOGIN PASSWORD 'recruitmate' CREATEDB;"
psql -d postgres -c "CREATE DATABASE recruitmate OWNER recruitmate;"
psql -d recruitmate -c "CREATE EXTENSION IF NOT EXISTS vector; CREATE EXTENSION IF NOT EXISTS pgcrypto;"
```

</details>

### 2. 启动后端

```bash
cd apps/api
cp .env.example .env     # 按需填写 AI Key
make migrate-up          # 自动迁移（服务启动时也会自动执行）
make seed                # 演示数据：3 个账号、3 个岗位、3 位候选人
make dev                 # http://localhost:8080
```

### 3. 启动前端

```bash
pnpm install
pnpm dev:web             # 内部端 http://localhost:5173
pnpm dev:careers         # 外部端 http://localhost:5174
```

### 演示账号（seed 数据）

| 角色 | 账号 | 密码 |
|---|---|---|
| 管理员 | admin@recruitmate.local | Recruitmate1! |
| HR | hr@recruitmate.local | Recruitmate1! |
| 部门负责人（技术部） | manager@tech.recruitmate.local | Recruitmate1! |

> 外部端「我的投递」使用邮箱验证码登录，验证码打印在后端日志（`code for xxx: 123456`）；生产环境可对接邮件服务。

## AI 配置（可选）

未配置时系统自动以**规则匹配模式**运行（技能命中/年限/学历规则打分）。配置后启用完整流水线：

```env
DEEPSEEK_API_KEY=sk-...        # LLM 评委与简历解析（OpenAI 兼容）
SILICONFLOW_API_KEY=sk-...     # embedding（BAAI/bge-m3，1024 维）
# 可选：DEEPSEEK_BASE_URL / DEEPSEEK_MODEL / EMBEDDING_MODEL / AI_TOPN / AI_*_W 权重
```

匹配综合分 = 0.45×LLM 评委 + 0.30×语义相似度 + 0.25×规则分（权重可配），每个评分附理由、技能命中/缺失与硬性条件逐项检查，HR 一眼看懂排序依据。

## 仓库结构

```
apps/api        Go 后端（Gin + pgx + asynq + Eino；migrations/、cmd/server|migrate|seed）
apps/web        内部端（Ant Design 5 + TanStack Query + Zustand）
apps/careers    外部端（Tailwind CSS）
packages/shared-types   前后端共享类型契约（单一事实来源）
packages/api-client     类型安全 API 客户端
docs/           技术设计文档
```

## 工程命令

```bash
make infra-up / infra-down   # 基础设施启停
make api-test                # 后端单测 + 集成测试
make build                   # 后端二进制 + 前端构建
pnpm -r typecheck / lint     # 前端静态检查
```

CI（GitHub Actions）：每个 PR 跑 Go vet/build/test（含 PostgreSQL/Redis service 容器）+ 前端 lint/typecheck/build。
