#!/usr/bin/env bash
# OA 流程全链路验证：状态机约束 + Offer 审批链（四眼原则）+ 时间线
set -euo pipefail
API="http://localhost:8080/api/v1"
PASS=0; FAIL=0
check() { if [ "$2" = "0" ]; then echo "  ✅ $1"; PASS=$((PASS+1)); else echo "  ❌ $1"; FAIL=$((FAIL+1)); fi; }

login() { curl -sf -X POST "$API/internal/auth/login" -H 'Content-Type: application/json' -d "{\"email\":\"$1\",\"password\":\"Recruitmate1!\"}" | python3 -c "import sys,json;print(json.load(sys.stdin)['token'])"; }
HR=$(login hr@recruitmate.local); MGR=$(login manager@tech.recruitmate.local); ADMIN=$(login admin@recruitmate.local)

# 找技术部 Go 岗位（部门负责人属于技术部）与一个 new 阶段候选人
JOB=$(curl -sf -H "Authorization: Bearer $HR" "$API/internal/jobs?pageSize=100" | python3 -c "
import sys,json
for j in json.load(sys.stdin)['items']:
    if '后端工程师' in j['title']: print(j['id']); break")
APP=$(curl -sf -H "Authorization: Bearer $HR" "$API/internal/jobs/$JOB/applications?stage=new&pageSize=1" | python3 -c "import sys,json; d=json.load(sys.stdin)['items']; print(d[0]['id'] if d else '')")
if [ -z "$APP" ]; then echo "无 new 阶段候选人，先造一个"; APPLY=$(curl -sf -X POST "$API/public/jobs/$JOB/applications" -F "name=OA测试" -F "email=oa-$(date +%s)@test.local" -F "phone=13900000000" -F "resumeText=5年Java经验，熟悉AWS与Kubernetes，本科。"); APP=$(echo "$APPLY" | python3 -c "import sys,json;print(json.load(sys.stdin)['id'])"); sleep 2; fi
echo "测试候选人: $APP"

# 1. 非法跳步 new→offer_pending 应 409
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X PATCH "$API/internal/applications/$APP/stage" -H "Authorization: Bearer $HR" -H 'Content-Type: application/json' -d '{"stage":"offer_pending"}')
check "非法跳步被拒(409)" "$([ "$CODE" = "409" ] && echo 0 || echo 1)"

# 2. 淘汰不带原因应 400
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X PATCH "$API/internal/applications/$APP/stage" -H "Authorization: Bearer $HR" -H 'Content-Type: application/json' -d '{"stage":"rejected"}')
check "淘汰缺原因被拒(400)" "$([ "$CODE" = "400" ] && echo 0 || echo 1)"

# 3. 正常流转 new→screening→interview(HR初面)→manager_interview(部门负责人面)
curl -sf -X PATCH "$API/internal/applications/$APP/stage" -H "Authorization: Bearer $HR" -H 'Content-Type: application/json' -d '{"stage":"screening"}' >/dev/null && check "初筛通过" 0 || check "初筛通过" 1
curl -sf -X PATCH "$API/internal/applications/$APP/stage" -H "Authorization: Bearer $HR" -H 'Content-Type: application/json' -d '{"stage":"interview","reason":"HR初面表现良好"}' >/dev/null && check "HR初面(带评价)" 0 || check "HR初面(带评价)" 1
curl -sf -X PATCH "$API/internal/applications/$APP/stage" -H "Authorization: Bearer $HR" -H 'Content-Type: application/json' -d '{"stage":"manager_interview","reason":"部门负责人面通过"}' >/dev/null && check "部门负责人面(带评价)" 0 || check "部门负责人面(带评价)" 1

# 4. HR 发起 Offer（建议薪资）
OFFER=$(curl -sf -X POST "$API/internal/applications/$APP/offer" -H "Authorization: Bearer $HR" -H 'Content-Type: application/json' -d '{"salary":"建议25K","joinDate":"2026-09-01","note":"按技术岗标准"}')
echo "$OFFER" | grep -q '"pending"' && check "HR 发起 Offer" 0 || check "HR 发起 Offer" 1

# 5. 四眼：HR 不能批自己的 Offer
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$API/internal/applications/$APP/offer/approve" -H "Authorization: Bearer $HR" -H 'Content-Type: application/json' -d '{"salary":"30K"}')
check "发起人自批被拒(403)" "$([ "$CODE" = "403" ] && echo 0 || echo 1)"

# 5b. 审批通过但不填薪资应 400（薪资由部门负责人确定）
CODE=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$API/internal/applications/$APP/offer/approve" -H "Authorization: Bearer $MGR" -H 'Content-Type: application/json' -d '{}')
check "审批缺最终薪资被拒(400)" "$([ "$CODE" = "400" ] && echo 0 || echo 1)"

# 6. 部门负责人审批通过并确定最终薪资
curl -sf -X POST "$API/internal/applications/$APP/offer/approve" -H "Authorization: Bearer $MGR" -H 'Content-Type: application/json' -d '{"salary":"28K"}' >/dev/null && check "部门负责人审批通过(定薪)" 0 || check "部门负责人审批通过(定薪)" 1

# 7. 终态验证：offered + 最终薪资 28K + 时间线
DETAIL=$(curl -sf -H "Authorization: Bearer $HR" "$API/internal/applications/$APP")
echo "$DETAIL" | python3 -c "
import sys,json
d=json.load(sys.stdin)
assert d['stage']=='offered', d['stage']
assert d['offer']['status']=='approved'
assert d['offer']['salary']=='28K', d['offer']['salary']
assert len(d['events'])>=5, len(d['events'])
print('  stage=offered, 最终薪资=%s, events=%d' % (d['offer']['salary'], len(d['events'])))" && check "终态/定薪/时间线" 0 || check "终态/定薪/时间线" 1

# 8. 求职者视角状态
CAND_TOKEN=$(curl -sf -X POST "$API/public/auth/email-code" -H 'Content-Type: application/json' -d '{"email":"oa-verify@test.local"}' >/dev/null; echo "")
echo "（求职者映射由单测覆盖）"

echo; echo "结果：通过 $PASS 项，失败 $FAIL 项"; [ "$FAIL" = "0" ]
