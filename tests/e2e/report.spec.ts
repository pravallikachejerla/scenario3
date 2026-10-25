import { test, expect } from '@playwright/test';

// The deliverable is a static HTML report at /failure-map-and-wave-1.html.
// These tests cover the main user flows: open the report, see the failure
// map, see the wave-1 proposal, navigate between sections via the in-page
// headings, and confirm the advisory status is visible.

test('opens the failure map and wave-1 report', async ({ page }) => {
  await page.goto('/failure-map-and-wave-1.html');
  await expect(page).toHaveTitle(/Failure Map Update and Final First-Wave Proposal/);
});

test('shows the advisory status banner', async ({ page }) => {
  await page.goto('/failure-map-and-wave-1.html');
  await expect(page.getByText('Advisory only', { exact: true })).toBeVisible();
  await expect(page.getByText('No code changed', { exact: true })).toBeVisible();
  await expect(page.getByText('No repository promotion', { exact: true })).toBeVisible();
  await expect(page.getByText('No implementation', { exact: true })).toBeVisible();
});

test('shows the constraints section with all ten constraints', async ({ page }) => {
  await page.goto('/failure-map-and-wave-1.html');
  const constraintsHeading = page.getByRole('heading', { name: '1. Constraints (re-stated, applied throughout)' });
  await expect(constraintsHeading).toBeVisible();
  const constraintsList = constraintsHeading.locator('xpath=following-sibling::ol[1]');
  await expect(constraintsList.getByRole('listitem')).toHaveCount(10);
});

test('shows the failure map condition register with all twelve conditions', async ({ page }) => {
  await page.goto('/failure-map-and-wave-1.html');
  const registerHeading = page.getByRole('heading', { name: '2.1 Condition register (12 conditions, 4 already repaired in baseline)' });
  await expect(registerHeading).toBeVisible();
  // Each condition ID appears in the table; assert all twelve are present.
  for (const id of ['STAB-01', 'STAB-02', 'STAB-03', 'STAB-04', 'STAB-05', 'STAB-06', 'STAB-07', 'STAB-08', 'STAB-09', 'STAB-10', 'STAB-11', 'STAB-12']) {
    await expect(page.getByRole('cell', { name: id, exact: true }).first()).toBeVisible();
  }
});

test('shows the prioritisation table with P1, P2, and P3 rows', async ({ page }) => {
  await page.goto('/failure-map-and-wave-1.html');
  const prioHeading = page.getByRole('heading', { name: /2\.4 Prioritisation/ });
  await expect(prioHeading).toBeVisible();
  await expect(page.getByText('P1', { exact: true }).first()).toBeVisible();
  await expect(page.getByText('P2', { exact: true }).first()).toBeVisible();
  await expect(page.getByText('P3', { exact: true }).first()).toBeVisible();
});

test('shows the final first-wave proposal with all five items', async ({ page }) => {
  await page.goto('/failure-map-and-wave-1.html');
  const waveHeading = page.getByRole('heading', { name: '3. Final First-Wave Proposal' });
  await expect(waveHeading).toBeVisible();
  for (const item of ['W1-1', 'W1-2', 'W1-3', 'W1-4', 'W1-5']) {
    await expect(page.getByRole('cell', { name: item, exact: true }).first()).toBeVisible();
  }
});

test('shows the GO/NO-GO and rollback sections', async ({ page }) => {
  await page.goto('/failure-map-and-wave-1.html');
  await expect(page.getByRole('heading', { name: '3.2 Evaluators and acceptance criteria (per wave-1 item)' })).toBeVisible();
  await expect(page.getByRole('heading', { name: '3.3 Rollback boundaries (per wave-1 item)' })).toBeVisible();
  await expect(page.getByRole('heading', { name: '3.4 What wave 1 does NOT do (explicit non-goals)' })).toBeVisible();
});

test('shows the unresolved-tension closing section', async ({ page }) => {
  await page.goto('/failure-map-and-wave-1.html');
  const closing = page.getByRole('heading', { name: '4. What would need to become true before any one path is safe' });
  await expect(closing).toBeVisible();
  await expect(page.getByText(/ship wave 1 as proposed/i)).toBeVisible();
  await expect(page.getByText(/wait for additional evidence/i)).toBeVisible();
  await expect(page.getByText(/escalate to a human decision-maker/i)).toBeVisible();
});
