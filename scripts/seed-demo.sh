#!/usr/bin/env bash
# 演示数据灌入：为技术部岗位（后端/前端）批量创建处于各流程阶段的候选人。
# 全部走真实 API 流程（投递→初筛→面试安排/评价→Offer→入职），
# 保证面试时间、评价、时间线等数据真实完整。
# 用法：bash scripts/seed-demo.sh [BASE_URL]   （默认 http://localhost:8080）
set -euo pipefail

BASE="${1:-http://localhost:8080}"
API="$BASE/api/v1"
RUN="$(date +%s)"

login() { curl -sf -X POST "$API/internal/auth/login" -H 'Content-Type: application/json' -d "{\"email\":\"$1\",\"password\":\"Recruitmate1!\"}" | python3 -c "import sys,json;print(json.load(sys.stdin)['token'])"; }
HR=$(login hr@recruitmate.local); MGR=$(login manager@tech.recruitmate.local); ADMIN=$(login admin@recruitmate.local)

iso_in() { python3 -c "from datetime import datetime,timedelta,timezone; print((datetime.now(timezone.utc)+timedelta(days=$1,hours=$2)).strftime('%Y-%m-%dT%H:%M:%SZ'))"; }

post() { curl -sf -X POST "$1" -H "Authorization: Bearer $2" -H 'Content-Type: application/json' -d "$3"; }
patch_stage() { curl -sf -X PATCH "$API/internal/applications/$1/stage" -H "Authorization: Bearer $HR" -H 'Content-Type: application/json' -d "{\"stage\":\"$2\"}" >/dev/null; }

# 查找技术部岗位 id
BACKEND_JOB=$(curl -sf -H "Authorization: Bearer $HR" "$API/internal/jobs?pageSize=100" | python3 -c "
import sys,json
for j in json.load(sys.stdin)['items']:
    if '后端工程师' in j['title']: print(j['id']); break")
FRONTEND_JOB=$(curl -sf -H "Authorization: Bearer $HR" "$API/internal/jobs?pageSize=100" | python3 -c "
import sys,json
for j in json.load(sys.stdin)['items']:
    if '前端工程师' in j['title']: print(j['id']); break")
echo "后端岗位: $BACKEND_JOB  前端岗位: $FRONTEND_JOB"

# 岗位未在招聘中则自动重新开放（演示数据需要）
for J in "$BACKEND_JOB" "$FRONTEND_JOB"; do
  ST=$(curl -sf -H "Authorization: Bearer $HR" "$API/internal/jobs/$J" | python3 -c "import sys,json;print(json.load(sys.stdin)['status'])")
  if [ "${ST}" != "open" ]; then
    echo "  ⚠️ 岗位 $J 状态 ${ST}，自动重新开放"
    curl -sf -X POST "$API/internal/jobs/$J/reopen" -H "Authorization: Bearer $HR" >/dev/null
  fi
done

# 已入职人数（避免满编自动关闭岗位）
HIRED_BACKEND=$(curl -sf -H "Authorization: Bearer $HR" "$API/internal/jobs/$BACKEND_JOB" | python3 -c "import sys,json;print(json.load(sys.stdin).get('hiredCount') or 0)")
HIRED_FRONTEND=$(curl -sf -H "Authorization: Bearer $HR" "$API/internal/jobs/$FRONTEND_JOB" | python3 -c "import sys,json;print(json.load(sys.stdin).get('hiredCount') or 0)")
echo "已入职：后端 $HIRED_BACKEND/2，前端 $HIRED_FRONTEND/2"

apply() { # $1=job $2=name $3=email $4=resume
  curl -sf -X POST "$API/public/jobs/$1/applications" \
    -F "name=$2" -F "email=$3" -F "phone=138$(printf '%08d' $((RANDOM % 100000000)))" -F "source=官网" -F "resumeText=$4" \
    | python3 -c "import sys,json;print(json.load(sys.stdin)['id'])"
}
schedule() { post "$API/internal/applications/$1/interviews" "$HR" "{\"round\":\"$2\",\"scheduledAt\":\"$3\"}" >/dev/null; }
complete() { post "$API/internal/applications/$1/interviews/$2/complete" "$3" "{\"result\":\"$4\",\"feedback\":\"$5\"}" >/dev/null; }
reject_with() { curl -sf -X PATCH "$API/internal/applications/$1/stage" -H "Authorization: Bearer $HR" -H 'Content-Type: application/json' -d "{\"stage\":\"rejected\",\"reason\":\"$2\"}" >/dev/null; }

# 流程推进助手
to_screening() { patch_stage "$1" screening; }
to_hr_interview() { to_screening "$1"; schedule "$1" hr "$(iso_in 1 10)"; }
finish_hr() { # $1=app $2=pass|fail $3=评价
  if [ "$2" = "pass" ]; then complete "$1" hr "$HR" pass "$3";
  else complete "$1" hr "$HR" fail "$3"; fi
}
to_manager_interview() { schedule "$1" manager "$(iso_in 3 14)"; }
finish_manager() { # $1=app $2=pass|fail $3=评价
  complete "$1" manager "$MGR" "$2" "$3"
}
to_offer_pending() { post "$API/internal/applications/$1/offer" "$HR" "{\"salary\":\"$2\",\"joinDate\":\"2026-10-$(printf '%02d' $((8 + RANDOM % 15)))\",\"note\":\"演示数据\"}" >/dev/null; }
approve_offer() { post "$API/internal/applications/$1/offer/approve" "$MGR" "{\"salary\":\"$2\"}" >/dev/null; }
to_hired() { curl -sf -X PATCH "$API/internal/applications/$1/stage" -H "Authorization: Bearer $HR" -H 'Content-Type: application/json' -d '{"stage":"hired"}' >/dev/null; }

E="demo2-$RUN"
COUNT=0
note() { COUNT=$((COUNT+1)); echo "  [$COUNT] $1"; }

echo "== 后端工程师（Go）=="
# 待初筛 new
for entry in "张伟:8年Go后端开发，精通微服务架构，主导过日均亿级请求系统，熟悉PostgreSQL分库分表与Redis集群。" \
             "刘洋:6年Java开发，近2年转向Go，熟悉Kubernetes与云原生体系，参与过金融支付系统建设。" \
             "杨帆:5年Go开发，专注云原生与可观测性，熟悉Prometheus与Grafana，主导过容器化改造。" \
             "赵磊:4年Go开发，擅长高并发与消息队列，熟悉Redis与PostgreSQL，参与过电商大促保障。" \
             "陈晨:2026届计算机硕士应届生，熟悉Go与Python，实习期间参与过微服务项目开发。"; do
  name="${entry%%:*}"; resume="${entry#*:}"
  app=$(apply "$BACKEND_JOB" "$name" "${name}-${E}@test.local" "姓名：${name}。${resume}")
  note "待初筛：${name}"
done

# 初筛通过（待安排HR面）
for entry in "黄晓明:10年架构经验，Go技术专家，主导过大型分布式系统与中间件平台建设，熟悉PostgreSQL与Redis。" \
             "周杰:7年Go开发，专注中间件与基础架构，熟悉Kubernetes与Service Mesh，开源社区活跃。"; do
  name="${entry%%:*}"; resume="${entry#*:}"
  app=$(apply "$BACKEND_JOB" "$name" "${name}-${E}@test.local" "姓名：${name}。${resume}")
  to_screening "$app"
  note "初筛通过：${name}"
done

# HR面已安排（待HR评价）
name="吴越"; app=$(apply "$BACKEND_JOB" "$name" "${name}-${E}@test.local" "姓名：${name}。5年Go后端经验，熟悉PostgreSQL与Redis，负责过订单系统重构。")
to_hr_interview "$app"; note "HR面待评价：${name}"

# HR面通过（待安排负责人面）
name="郑凯"; app=$(apply "$BACKEND_JOB" "$name" "${name}-${E}@test.local" "姓名：${name}。资深Go工程师，7年经验，微服务与数据库优化经验丰富。")
to_hr_interview "$app"; finish_hr "$app" pass "技术功底扎实，系统设计思路清晰，建议进入负责人面"
note "待安排负责人面：${name}"

# 负责人面已安排（待负责人评价）
name="孙倩"; app=$(apply "$BACKEND_JOB" "$name" "${name}-${E}@test.local" "姓名：${name}。6年Go开发，精通PostgreSQL与Redis，有团队管理经验。")
to_hr_interview "$app"; finish_hr "$app" pass "沟通良好，项目经验匹配"
to_manager_interview "$app"; note "负责人面待评价：${name}"

# 负责人面通过（待发起Offer）
name="李强"; app=$(apply "$BACKEND_JOB" "$name" "${name}-${E}@test.local" "姓名：${name}。资深架构师，9年经验，主导过中台架构演进，精通Go与PostgreSQL。")
to_hr_interview "$app"; finish_hr "$app" pass "架构能力突出"
to_manager_interview "$app"; finish_manager "$app" pass "专业能力与团队匹配度高，同意录用"
note "待发起Offer：${name}"

# Offer 审批中
name="马超"; app=$(apply "$BACKEND_JOB" "$name" "${name}-${E}@test.local" "姓名：${name}。8年Go后端，云原生经验丰富，熟悉Kubernetes与微服务治理。")
to_hr_interview "$app"; finish_hr "$app" pass "经验丰富，技术面优秀"
to_manager_interview "$app"; finish_manager "$app" pass "符合岗位要求，同意录用"
to_offer_pending "$app" "建议25"; note "Offer审批中：${name}"

# 已发Offer
name="何洁"; app=$(apply "$BACKEND_JOB" "$name" "${name}-${E}@test.local" "姓名：${name}。7年Go开发，专注支付与账务系统，PostgreSQL优化经验丰富。")
to_hr_interview "$app"; finish_hr "$app" pass "业务理解深入"
to_manager_interview "$app"; finish_manager "$app" pass "同意录用"
to_offer_pending "$app" "26"; approve_offer "$app" "27"; note "已发Offer：${name}"

# 已入职（控制数量避免满编）
if [ "$HIRED_BACKEND" -lt 2 ]; then
  name="罗志"; app=$(apply "$BACKEND_JOB" "$name" "${name}-${E}@test.local" "姓名：${name}。Go技术专家，10年经验，高并发系统与数据库调优专家。")
  to_hr_interview "$app"; finish_hr "$app" pass "技术专家级"
  to_manager_interview "$app"; finish_manager "$app" pass "同意录用"
  to_offer_pending "$app" "30"; approve_offer "$app" "32"; to_hired "$app"
  note "已入职：${name}"
else
  note "跳过已入职（后端已满编）"
fi

# 已淘汰（初筛淘汰）
name="韩雪"; app=$(apply "$BACKEND_JOB" "$name" "${name}-${E}@test.local" "姓名：${name}。应届生，自学Go两个月，熟悉基础语法。")
reject_with "$app" "经验与岗位要求差距较大"; note "已淘汰：${name}"

echo "== 前端工程师 =="
for entry in "林芳:6年React开发经验，熟悉TypeScript与组件库建设，主导过中后台系统重构。" \
             "高原:5年全栈开发，React与Node.js经验丰富，熟悉微前端架构。" \
             "邓超:2026届本科应届生，熟悉React与TypeScript，实习参与过管理后台开发。"; do
  name="${entry%%:*}"; resume="${entry#*:}"
  app=$(apply "$FRONTEND_JOB" "$name" "${name}-${E}@test.local" "姓名：${name}。${resume}")
  note "待初筛：${name}"
done

name="宋佳"; app=$(apply "$FRONTEND_JOB" "$name" "${name}-${E}@test.local" "姓名：${name}。4年前端，Vue转React一年，熟悉TypeScript。")
to_screening "$app"; note "初筛通过：${name}"

name="蒋明"; app=$(apply "$FRONTEND_JOB" "$name" "${name}-${E}@test.local" "姓名：${name}。5年前端，React与TypeScript熟练，有可视化大屏经验。")
to_hr_interview "$app"; note "HR面待评价：${name}"

name="蔡琴"; app=$(apply "$FRONTEND_JOB" "$name" "${name}-${E}@test.local" "姓名：${name}。资深前端，8年经验，组件库与工程化负责人。")
to_hr_interview "$app"; finish_hr "$app" pass "工程化能力突出"
to_manager_interview "$app"; note "负责人面待评价：${name}"

name="魏晨"; app=$(apply "$FRONTEND_JOB" "$name" "${name}-${E}@test.local" "姓名：${name}。7年前端，React专家，性能优化与微前端经验丰富。")
to_hr_interview "$app"; finish_hr "$app" pass "技术能力强"
to_manager_interview "$app"; finish_manager "$app" pass "同意录用"
to_offer_pending "$app" "24"; note "Offer审批中：${name}"

if [ "$HIRED_FRONTEND" -lt 2 ]; then
  name="沈月"; app=$(apply "$FRONTEND_JOB" "$name" "${name}-${E}@test.local" "姓名：${name}。前端专家，9年经验，主导过大型前端架构升级。")
  to_hr_interview "$app"; finish_hr "$app" pass "前端专家级"
  to_manager_interview "$app"; finish_manager "$app" pass "同意录用"
  to_offer_pending "$app" "28"; approve_offer "$app" "30"; to_hired "$app"
  note "已入职：${name}"
else
  note "跳过已入职（前端已满编）"
fi

name="彭飞"; app=$(apply "$FRONTEND_JOB" "$name" "${name}-${E}@test.local" "姓名：${name}。2年PHP开发，想转前端，React了解不多。")
to_screening "$app"; schedule "$app" hr "$(iso_in 1 10)"
complete "$app" hr "$HR" fail "前端基础薄弱，与岗位要求差距较大"
note "面试未通过淘汰：$name"

echo
echo "✅ 演示数据灌入完成：共 $COUNT 位候选人，分布于各流程阶段。"
echo "AI 评分在后台异步进行，约 1-3 分钟后全部出分。"
