import { test, expect } from '@playwright/test';

test('app shell renders with header and navigation', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByRole('heading', { name: /Order Management/i })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Orders' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Wave-1 Evidence' })).toBeVisible();
});

test('Orders tab is the default and shows the order search panel', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByRole('heading', { name: 'Order Search' })).toBeVisible();
  await expect(page.getByLabel(/Tenant/i)).toBeVisible();
});

test('user can navigate to the Wave-1 Evidence tab', async ({ page }) => {
  await page.goto('/');
  await page.getByRole('button', { name: 'Wave-1 Evidence' }).click();
  await expect(page.getByRole('heading', { name: /Wave-1 Evidence/i })).toBeVisible();
});

test('user can navigate to the Inventory tab', async ({ page }) => {
  await page.goto('/');
  await page.getByRole('button', { name: 'Inventory' }).click();
  await expect(page.getByRole('heading', { name: /Inventory/i })).toBeVisible();
});

test('user can navigate to the Configuration tab', async ({ page }) => {
  await page.goto('/');
  await page.getByRole('button', { name: 'Configuration' }).click();
  await expect(page.getByRole('heading', { name: /Configuration/i })).toBeVisible();
});

test('user can navigate to the Incident Panel tab', async ({ page }) => {
  await page.goto('/');
  await page.getByRole('button', { name: 'Incident Panel' }).click();
  await expect(page.getByRole('heading', { name: /Incident/i })).toBeVisible();
});