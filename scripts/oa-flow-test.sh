#!/usr/bin/env bash
# OA 流程全链路验证（面试实体版）：初筛 → 安排HR面 → HR面评价 → 安排负责人面
# → 负责人面评价 → 发起Offer → 审批定薪 → 入职 → 满编自动关闭
set -euo pipefail
API="${1:-http://localhost:8080}/api/v1"
PASS=0; FAIL=0
check() { if [ "$2" = "0" ]; then echo "  ✅ $1"; PASS=$((PASS+1)); else echo "  ❌ $1"; FAIL=$((FAIL+1)); fi; }

login() { curl -sf -X POST "$API/internal/auth/login" -H 'Content-Type: application/json' -d "{\"email\":\"$1\",\"password\":\"Recruitmate1!\"}" | python3 -c "import sys,json;print(json.load(sys.stdin)['token'])"; }
HR=$(login hr@recruitmate.local); MGR=$(login manager@tech.recruitmate.local); ADMIN=$(login admin@recruitmate.local)

# 技术部 Go 岗位与一个 new 阶段候选人
JOB=$(curl -sf -H "Authorization: Bearer $HR" "$API/internal/jobs?pageSize=100" | python3 -c "
import sys,json
for j in json.load(sys.stdin)['items']:
    if '后端工程师' in j['title'] and j['status']=='open': print(j['id']); break")
APP=$(curl -sf -H "Authorization: Bearer $HR" "$API/internal/jobs/$JOB/applications?stage=new&pageSize=1" | python3 -c "import sys,json; d=json.load(sys.stdin)['items']; print(d[0]['id'] if d else '')")
if [ -z "$APP" ]; then
  APPLY=$(curl -sf -X POST "$API/public/jobs/$JOB/applications" -F "name=OA测试" -F "email=oa-$(date +%s)@test.local" -F "phone=13900000000" -F "resumeText=5年Java经验，熟悉AWS与Kubernetes，本科。")
  APP=$(echo "$APPLY" | python3 -c "import sys,json;print(json.load(sys.stdin)['id'])"); sleep 2
fi
echo "测试候选人: $APP"

# 1. 手动跳转面试阶段应被拒（阶段由面试动作驱动）
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X PATCH "$API/internal/applications/$APP/stage" -H "Authorization: Bearer $HR" -H 'Content-Type: application/json' -d '{"stage":"interview"}')
check "手动跳转 HR 面被拒(409)" "$([ "$CODE" = "409" ] && echo 0 || echo 1)"

# 2. 初筛通过
curl -sf -X PATCH "$API/internal/applications/$APP/stage" -H "Authorization: Bearer $HR" -H 'Content-Type: application/json' -d '{"stage":"screening"}' >/dev/null && check "初筛通过" 0 || check "初筛通过" 1

# 3. 安排 HR 面（时间必填，推进到 interview）
curl -sf -X POST "$API/internal/applications/$APP/interviews" -H "Authorization: Bearer $HR" -H 'Content-Type: application/json' \
  -d '{"round":"hr","scheduledAt":"2026-08-25T10:00:00+08:00"}' >/dev/null && check "安排 HR 面" 0 || check "安排 HR 面" 1

# 3b. 未通过 HR 面不能安排负责人面
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$API/internal/applications/$APP/interviews" -H "Authorization: Bearer $HR" -H 'Content-Type: application/json' \
  -d '{"round":"manager","scheduledAt":"2026-08-26T10:00:00+08:00"}')
check "HR 面未通过不能安排负责人面(409)" "$([ "$CODE" = "409" ] && echo 0 || echo 1)"

# 4. HR 完成面试（评价必填 + 通过）
curl -sf -X POST "$API/internal/applications/$APP/interviews/hr/complete" -H "Authorization: Bearer $HR" -H 'Content-Type: application/json' \
  -d '{"result":"pass","feedback":"技术扎实，沟通良好，建议进入负责人面"}' >/dev/null && check "HR 面完成(通过)" 0 || check "HR 面完成(通过)" 1

# 5. 安排负责人面 → manager_interview
curl -sf -X POST "$API/internal/applications/$APP/interviews" -H "Authorization: Bearer $HR" -H 'Content-Type: application/json' \
  -d '{"round":"manager","scheduledAt":"2026-08-26T14:00:00+08:00"}' >/dev/null && check "安排负责人面" 0 || check "安排负责人面" 1

# 5b. HR 不能完成负责人面评价（角色分工）
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$API/internal/applications/$APP/interviews/manager/complete" -H "Authorization: Bearer $HR" -H 'Content-Type: application/json' \
  -d '{"result":"pass","feedback":"x"}')
check "HR 越权完成负责人面被拒(403)" "$([ "$CODE" = "403" ] && echo 0 || echo 1)"

# 6. 部门负责人完成负责人面（评价 + 通过）
curl -sf -X POST "$API/internal/applications/$APP/interviews/manager/complete" -H "Authorization: Bearer $MGR" -H 'Content-Type: application/json' \
  -d '{"result":"pass","feedback":"专业能力符合团队要求，同意录用"}' >/dev/null && check "负责人面完成(通过)" 0 || check "负责人面完成(通过)" 1

# 6b. 负责人面未通过不能发起 Offer（本处已通过，跳过；验证待办接口可用）
curl -sf -H "Authorization: Bearer $HR" "$API/internal/todos/interviews" | grep -q '"items"' && check "我的待办接口" 0 || check "我的待办接口" 1
curl -sf -H "Authorization: Bearer $HR" "$API/internal/candidates?pageSize=5" | grep -q '"items"' && check "候选人中心接口" 0 || check "候选人中心接口" 1

# 7. HR 发起 Offer（建议薪资）
OFFER=$(curl -sf -X POST "$API/internal/applications/$APP/offer" -H "Authorization: Bearer $HR" -H 'Content-Type: application/json' -d '{"salary":"建议25","joinDate":"2026-09-01","note":"按技术岗标准"}')
echo "$OFFER" | grep -q '"pending"' && check "HR 发起 Offer" 0 || check "HR 发起 Offer" 1

# 8. 四眼 + 缺定薪校验
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$API/internal/applications/$APP/offer/approve" -H "Authorization: Bearer $HR" -H 'Content-Type: application/json' -d '{"salary":"30"}')
check "发起人自批被拒(403)" "$([ "$CODE" = "403" ] && echo 0 || echo 1)"
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$API/internal/applications/$APP/offer/approve" -H "Authorization: Bearer $MGR" -H 'Content-Type: application/json' -d '{}')
check "审批缺最终薪资被拒(400)" "$([ "$CODE" = "400" ] && echo 0 || echo 1)"

# 9. 部门负责人审批通过并定薪
curl -sf -X POST "$API/internal/applications/$APP/offer/approve" -H "Authorization: Bearer $MGR" -H 'Content-Type: application/json' -d '{"salary":"28"}' >/dev/null && check "审批通过(定薪)" 0 || check "审批通过(定薪)" 1

# 10. 终态校验：面试记录/时间线/定薪
curl -sf -H "Authorization: Bearer $HR" "$API/internal/applications/$APP" | python3 -c "
import sys,json
d=json.load(sys.stdin)
assert d['stage']=='offered', d['stage']
assert d['offer']['salary']=='28'
ivs={i['round']:i for i in d.get('interviews',[])}
assert ivs['hr']['result']=='pass' and ivs['hr']['feedback']!=''
assert ivs['manager']['result']=='pass' and ivs['manager']['scheduledAt'] is not None
assert len(d.get('events',[]))>=6
print('  两轮面试记录完整，时间线 %d 条' % len(d['events']))" && check "面试记录/时间线/定薪" 0 || check "面试记录/时间线/定薪" 1

echo; echo "结果：通过 $PASS 项，失败 $FAIL 项"; [ "$FAIL" = "0" ]
