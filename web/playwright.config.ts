import { defineConfig, devices } from "@playwright/test";
export default defineConfig({
  testDir: "./e2e",
  fullyParallel: false,
  workers: 1,
  timeout: 30000,
  use: { baseURL: "http://localhost:13000", trace: "retain-on-failure" },
  webServer: {
    command: "bun run start --port 13000",
    url: "http://localhost:13000/healthz",
    reuseExistingServer: !process.env.CI,
    timeout: 120000,
    env: { BIODAYS_SITE_URL: "http://localhost:13000", BIODAYS_DISABLE_ANALYTICS: "true" },
  },
  projects: [
    { name: "desktop", use: { ...devices["Desktop Chrome"] } },
    { name: "mobile", use: { ...devices["iPhone 13"], defaultBrowserType: "chromium" } },
  ],
});
