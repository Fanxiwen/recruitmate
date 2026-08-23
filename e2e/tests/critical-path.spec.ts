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
  await expect(page.getByText('Recruitmate', { exact: false }).first()).toBeVisible();

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
  await page.getByLabel(/简历|粘贴/).fill(
    '姓名：E2E测试候选人。6 年 Go 后端开发经验，精通 PostgreSQL 与 Redis，本科毕业于某大学计算机系。',
  );
  await page.getByRole('button', { name: /提交/ }).click();
  await expect(page.getByText(/投递成功|已投递|成功/)).toBeVisible({ timeout: 15_000 });
});

test('内部端：HR 登录并查看候选人（匹配度排序 + 匹配报告）', async ({ page }) => {
  await page.goto(`${WEB}/login`);
  await page.getByLabel(/邮箱/).fill(HR_EMAIL);
  await page.getByLabel(/密码/).fill(HR_PASSWORD);
  await page.getByRole('button', { name: /登录/ }).click();
  await expect(page).toHaveURL(/\/jobs/, { timeout: 15_000 });

  // 进入第一个岗位的候选人 Tab
  await page.locator('table a, [data-testid="job-row-link"], a[href^="/jobs/"]').first().click();
  await expect(page.getByText(/候选人/)).toBeVisible({ timeout: 15_000 });

  // 候选人表格有数据行，且存在匹配度信息（评分中 或 数字）
  await expect(page.locator('tbody tr').first()).toBeVisible({ timeout: 15_000 });
  await expect(page.locator('tbody tr').first().getByText(/评分中|\d+/).first()).toBeVisible();

  // 打开匹配报告抽屉
  await page.locator('tbody tr').first().click();
  await expect(page.getByText(/匹配报告|综合分/)).toBeVisible({ timeout: 15_000 });
  await page.keyboard.press('Escape');

  // 行内流转阶段（选中该行任一 Select）
  const stageSelect = page.locator('tbody tr').first().locator('.ant-select').first();
  await stageSelect.click();
  await page.locator('.ant-select-dropdown').getByText(/初筛通过/).first().click();
  await expect(page.getByText(/已更新|成功|初筛通过/).first()).toBeVisible({ timeout: 15_000 });
});
