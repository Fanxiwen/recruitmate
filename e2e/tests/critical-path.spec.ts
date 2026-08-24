import { expect, test } from '@playwright/test';

/**
 * 核心闭环 E2E：求职者浏览并投递 → HR 登录查看候选人（AI 匹配度排序）→ 流转阶段。
 */

const WEB = 'http://localhost:5173';
const CAREERS = 'http://localhost:5174';
const HR_EMAIL = 'hr@recruitmate.local';
const HR_PASSWORD = 'Recruitmate1!';
const CANDIDATE_EMAIL = `e2e-${Date.now()}@test.local`;

test.describe.configure({ mode: 'serial' });

test('外部端：浏览岗位并投递简历', async ({ page }) => {
  await page.goto(CAREERS);
  await expect(page.getByText('中葡经贸中心', { exact: false }).first()).toBeVisible();

  // 岗位列表加载出卡片
  const cards = page.locator('a[href^="/jobs/"], [data-testid="job-card"]');
  await expect(cards.first()).toBeVisible({ timeout: 15_000 });
  const jobHref = await page.locator('a[href^="/jobs/"]').first().getAttribute('href');
  expect(jobHref).toBeTruthy();

  // 进入详情并投递（粘贴文本简历）
  await page.locator('a[href^="/jobs/"]').first().click();
  await expect(page.getByRole('button', { name: /投递|立即投递/ })).toBeVisible();
  await page.getByRole('button', { name: /投递|立即投递/ }).click();

  await page.getByLabel(/姓名/).fill('E2E测试候选人');
  await page.getByLabel(/邮箱/).fill(CANDIDATE_EMAIL);
  await page.getByLabel(/手机/).fill('13800000000');
  await page.locator('#apply-resume-text').fill(
    '姓名：E2E测试候选人。6 年 Go 后端开发经验，精通 PostgreSQL 与 Redis，本科毕业于某大学计算机系。',
  );
  await page.getByRole('button', { name: /提交/ }).click();
  await expect(page.getByText(/投递成功|已投递|成功/)).toBeVisible({ timeout: 15_000 });
});

test('内部端：HR 登录并查看候选人（匹配度排序 + 匹配报告）', async ({ page, request }) => {
  await page.goto(`${WEB}/login`);
  await page.getByLabel(/邮箱/).fill(HR_EMAIL);
  await page.getByLabel(/密码/).fill(HR_PASSWORD);
  // AntD 会在两个汉字的按钮文案中自动插入空格（"登 录"）
  await page.getByRole('button', { name: /登\s*录/ }).click();
  await expect(page).toHaveURL(/\/jobs/, { timeout: 15_000 });

  // 从本地认证态取 token，经 API 定位种子岗位（不依赖列表分页位置）
  const token = await page.evaluate(() => {
    const raw = localStorage.getItem('recruitmate-auth');
    return raw ? (JSON.parse(raw).state?.token as string) : null;
  });
  expect(token).toBeTruthy();
  const jobsRes = await request.get('http://localhost:8080/api/v1/internal/jobs?pageSize=100', {
    headers: { Authorization: `Bearer ${token}` },
  });
  expect(jobsRes.ok()).toBeTruthy();
  const jobs = (await jobsRes.json()) as { items: { id: string; title: string }[] };
  const seedJob = jobs.items.find((j) => j.title.includes('后端工程师'));
  expect(seedJob).toBeTruthy();
  await page.goto(`${WEB}/jobs/${seedJob!.id}`);

  await expect(page.getByRole('tab', { name: '候选人' })).toBeVisible({ timeout: 15_000 });

  // 候选人表格有数据行（.ant-table-row 为 AntD 数据行；measure row 是隐藏测量行）
  const firstRow = page.locator('tbody tr.ant-table-row').first();
  await expect(firstRow).toBeVisible({ timeout: 15_000 });
  await expect(firstRow.getByText(/评分中|\d+/).first()).toBeVisible();

  // 打开匹配报告抽屉（可解释 AI：综合分 + 理由）
  // 曾发现真实 bug：wrapper 的 onMouseDown focus() 触发浏览器滚动、sticky 列位移导致
  // 第一次点击失效 —— 已在 ApplicationTable 修复（focus({preventScroll:true})），
  // 这里用普通点击作为该修复的回归验证。
  await firstRow.getByRole('button', { name: '详情' }).click();
  await expect(page.getByText('AI 匹配报告')).toBeVisible({ timeout: 15_000 });
  await page.keyboard.press('Escape');
  await expect(page.getByText('AI 匹配报告')).toBeHidden();

  // 行内流转阶段：新简历 → 初筛通过
  const stageSelect = firstRow.locator('.ant-select').first();
  await stageSelect.click();
  await page.locator('.ant-select-dropdown').getByText('初筛通过').first().click();
  await expect(firstRow.getByTitle('初筛通过')).toBeVisible({ timeout: 15_000 });
});
