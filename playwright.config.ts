import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './e2e-playwright',
  fullyParallel: false,
  forbidOnly: true,
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  reporter: 'list',
  timeout: 60_000,
  use: {
    baseURL: 'http://localhost:5173',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
    actionTimeout: 10_000,
    navigationTimeout: 15_000,
  },
  webServer: {
    command: 'npm run dev -- --port 5173',
    url: 'http://localhost:5173/healthz',
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
});
