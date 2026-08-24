# 招聘流程设计（OA 化）

> 目标：让招聘流程清晰、严谨、可追溯。核心三原则：
> 1. **状态机约束**：阶段流转必须合法，服务端强制，不允许跳步；
> 2. **四眼原则**：发起人不能审批自己发起的流程（岗位发布、Offer）；
> 3. **全链路留痕**：每一次流转记录「谁、何时、从哪到哪、原因」，可回溯。

## 一、角色职责

| 角色 | 职责 | 权限边界 |
|---|---|---|
| 部门负责人（Hiring Manager） | 发起用人需求（岗位草稿）；审批本部门岗位发布；审批本部门候选人 Offer；填写面试评价 | 仅本部门数据（岗位/候选人/Offer） |
| HR | 初筛简历；安排与记录面试；发起 Offer 审批；维护候选人状态；全公司数据可见 | 全量数据；但不能审批自己发起的 Offer |
| 管理员 | 兜底审批（岗位、Offer）；账号与系统配置 | 全部 |
| 求职者 | 投递简历；查看投递进度 | 只能看自己、只能看到安全的状态映射 |

## 二、岗位发布流程（四眼）

```
部门负责人（或 HR）创建草稿
  → 提交审批（pending）
  → 审批人：本部门负责人 / 管理员
     —— 提交人自己不能批（四眼原则，admin 例外）
  → 通过（open，发布到外部端）/ 驳回（回 draft，需原因）
```

## 三、候选人流程（8 阶段状态机）

| 阶段 | 含义 | 谁操作 | 合法流转 |
|---|---|---|---|
| `new` | 新简历（待初筛） | HR | → screening / rejected |
| `screening` | 初筛通过 | HR | → interview / rejected |
| `interview` | HR 初面 | HR（记录评价） | → manager_interview / rejected |
| `manager_interview` | 部门负责人面 | HR（记录评价） | → offer_pending / rejected |
| `offer_pending` | Offer 审批中 | 部门负责人/管理员审批 | → offered（通过）/ manager_interview（驳回） |
| `offered` | 已发 Offer | HR | → hired / rejected |
| `hired` | 已入职 | HR | 终态 |
| `rejected` | 已淘汰 | HR | → new（误杀恢复，需原因） |

约束：
- **面试分两轮**：HR 初面 → 部门负责人面，不能跳轮次；
- 转 `rejected` **必须填淘汰原因**；进面试轮次建议填评价（记录在时间线）；
- Offer 审批：HR/管理员发起（建议薪资、入职时间、备注），审批人=**本部门负责人或管理员，且不能是发起人**；**审批通过时由部门负责人确定最终薪资（必填）**；驳回回退 `manager_interview` 并记录原因。

## 四、Offer 审批链（薪资由部门负责人决定）

```
HR 发起 Offer（offer_pending，建议薪资可选）
  → 部门负责人 / 管理员审批
     ├─ 通过（确定最终薪资，必填）→ offered（候选人可见：Offer 待确认）
     └─ 驳回（原因）→ manager_interview（回到部门负责人面）
```

## 五、求职者可见状态映射（不暴露内部细节）

| 内部阶段 | 求职者看到 |
|---|---|
| new | 已投递，处理中 |
| screening | 初筛通过 |
| interview / manager_interview | 面试邀请（两轮面试统一展示） |
| offer_pending / offered | Offer 沟通中 |
| hired | 已入职 |
| rejected | 未通过（不显示内部原因） |

## 六、时间线（application_events）

每次流转记录：application_id、from_stage、to_stage、action（stage_change / offer_request / offer_approve / offer_reject / feedback）、操作人、原因/备注、时间。HR 端候选人详情抽屉以 Timeline 展示。

## 七、数据模型（migration 00002）

- `applications` 增加：`reject_reason text`（淘汰原因）、`interview_feedback text`（面试评价）
- 新表 `offers`：application_id、salary、join_date、note、status(pending/approved/rejected)、requested_by、decided_by、requested_at、decided_at
- 新表 `application_events`：流转时间线（见上）
