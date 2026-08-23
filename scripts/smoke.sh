#!/usr/bin/env bash
# recruitmate 端到端冒烟测试：验证核心业务闭环
# 前置：后端已启动（make api-seed && make api-dev）
# 用法：bash scripts/smoke.sh [BASE_URL]   （默认 http://localhost:8080）
set -euo pipefail

BASE="${1:-http://localhost:8080}"
API="$BASE/api/v1"
PASS=0
FAIL=0

check() {
  local name="$1" ok="$2"
  if [ "$ok" = "0" ]; then
    echo "  ✅ $name"
    PASS=$((PASS + 1))
  else
    echo "  ❌ $name"
    FAIL=$((FAIL + 1))
  fi
}

echo "== 1. 外部端 =="

# 公开岗位列表
JOBS=$(curl -sf "$API/public/jobs?pageSize=10" || true)
echo "$JOBS" | grep -q '"items"' && check "公开岗位列表" 0 || check "公开岗位列表" 1

JOB_ID=$(echo "$JOBS" | python3 -c "import sys,json;print(json.load(sys.stdin)['items'][0]['id'])" 2>/dev/null || true)
[ -n "$JOB_ID" ] && check "取到第一个岗位 id" 0 || check "取到第一个岗位 id" 1

# 岗位详情
curl -sf "$API/public/jobs/$JOB_ID" | grep -q '"requirements"' && check "岗位详情" 0 || check "岗位详情" 1

# 投递（粘贴文本简历）
CANDIDATE_EMAIL="smoke-$(date +%s)@test.local"
APPLY=$(curl -sf -X POST "$API/public/jobs/$JOB_ID/applications" \
  -F "name=冒烟测试" -F "email=$CANDIDATE_EMAIL" -F "phone=13800000000" \
  -F "source=smoke" -F "resumeText=姓名：冒烟测试。5年Go开发经验，熟悉PostgreSQL与Redis，本科毕业于某大学。" || true)
echo "$APPLY" | grep -q '"id"' && check "投递简历（文本）" 0 || check "投递简历（文本）" 1

# 重复投递（同邮箱同岗位）应 409
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$API/public/jobs/$JOB_ID/applications" \
  -F "name=冒烟测试" -F "email=$CANDIDATE_EMAIL" -F "phone=13800000000" -F "resumeText=x")
check "重复投递防护(409)" "$([ "$CODE" = "409" ] && echo 0 || echo 1)"

# 候选人验证码登录（时间戳邮箱避免 60s 限流）
EMAIL="candidate-smoke-$(date +%s)@test.local"
curl -sf -X POST "$API/public/auth/email-code" -H 'Content-Type: application/json' -d "{\"email\":\"$EMAIL\"}" >/dev/null && check "发送邮箱验证码" 0 || check "发送邮箱验证码" 1
# 验证码打印在后端日志；此处直接断言错误码路径可用
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$API/public/auth/verify" -H 'Content-Type: application/json' -d "{\"email\":\"$EMAIL\",\"code\":\"000000\"}")
check "验证码校验接口可达" "$([ "$CODE" != "500" ] && echo 0 || echo 1)"

echo "== 2. 内部端 =="

# 登录
TOKEN=$(curl -sf -X POST "$API/internal/auth/login" -H 'Content-Type: application/json' \
  -d '{"email":"hr@recruitmate.local","password":"Recruitmate1!"}' | python3 -c "import sys,json;print(json.load(sys.stdin)['token'])" 2>/dev/null || true)
[ -n "$TOKEN" ] && check "HR 登录" 0 || check "HR 登录" 1
AUTH="Authorization: Bearer $TOKEN"

# 部门列表
curl -sf -H "$AUTH" "$API/internal/departments" | grep -q '"name"' && check "部门列表" 0 || check "部门列表" 1

# 创建岗位
DEPT_ID=$(curl -sf -H "$AUTH" "$API/internal/departments" | python3 -c "import sys,json;print(json.load(sys.stdin)[0]['id'])" 2>/dev/null || true)
NEWJOB=$(curl -sf -X POST "$API/internal/jobs" -H "$AUTH" -H 'Content-Type: application/json' \
  -d "{\"title\":\"冒烟测试岗位\",\"departmentId\":\"$DEPT_ID\",\"location\":\"北京\",\"jobType\":\"full_time\",\"headcount\":1,\"description\":\"测试\",\"requirements\":{\"mustSkills\":[\"Go\"],\"niceSkills\":[],\"minEducation\":\"bachelor\",\"minYears\":3}}" || true)
echo "$NEWJOB" | grep -q '"draft"' && check "创建草稿岗位" 0 || check "创建草稿岗位" 1
NEWJOB_ID=$(echo "$NEWJOB" | python3 -c "import sys,json;print(json.load(sys.stdin)['id'])" 2>/dev/null || true)

# 提交审批（HR）→ 审批通过（管理员：按设计审批权在管理员/部门负责人）
curl -sf -X POST "$API/internal/jobs/$NEWJOB_ID/submit" -H "$AUTH" >/dev/null && check "提交审批" 0 || check "提交审批" 1
ADMIN_TOKEN=$(curl -sf -X POST "$API/internal/auth/login" -H 'Content-Type: application/json' \
  -d '{"email":"admin@recruitmate.local","password":"Recruitmate1!"}' | python3 -c "import sys,json;print(json.load(sys.stdin)['token'])" 2>/dev/null || true)
[ -n "$ADMIN_TOKEN" ] && check "管理员登录" 0 || check "管理员登录" 1
curl -sf -X POST "$API/internal/jobs/$NEWJOB_ID/approve" -H "Authorization: Bearer $ADMIN_TOKEN" >/dev/null && check "审批通过" 0 || check "审批通过" 1

# 候选人列表（默认按匹配度排序）
APPS=$(curl -sf -H "$AUTH" "$API/internal/jobs/$JOB_ID/applications" || true)
echo "$APPS" | grep -q '"items"' && check "候选人列表" 0 || check "候选人列表" 1

# 岗位统计
curl -sf -H "$AUTH" "$API/internal/jobs/$JOB_ID/stats" | grep -q '"total"' && check "岗位统计" 0 || check "岗位统计" 1

echo
echo "结果：通过 $PASS 项，失败 $FAIL 项"
[ "$FAIL" = "0" ]
