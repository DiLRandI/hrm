import { test, expect } from '@playwright/test';

const baseURL = process.env.E2E_BASE_URL;

test('login screen loads', async ({ page }) => {
  test.skip(!baseURL, 'E2E_BASE_URL not set');
  await page.goto(baseURL);
  await expect(page.getByRole('button', { name: /sign in/i })).toBeVisible();
});
