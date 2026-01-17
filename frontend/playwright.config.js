import { defineConfig } from '@playwright/test';

const baseURL = process.env.E2E_BASE_URL;

export default defineConfig({
  testDir: './e2e',
  timeout: 30_000,
  retries: 0,
  use: {
    baseURL: baseURL || 'http://localhost:8080',
    headless: true,
  },
  reporter: 'list',
});
