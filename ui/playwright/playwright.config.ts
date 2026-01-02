import path from "path"
import { fileURLToPath } from "url"

import { defineConfig } from "@playwright/test"

const currentFilePath = fileURLToPath(import.meta.url)
const currentDir = path.dirname(currentFilePath)
const storageStatePath = path.join(currentDir, "storage", "admin.json")
const baseURL = process.env.E2E_BASE_URL || "http://localhost:8080"

export default defineConfig({
  testDir: path.join(currentDir, "tests"),
  outputDir: path.join(currentDir, "test-results"),
  fullyParallel: true,
  use: {
    baseURL,
    trace: "on-first-retry",
    headless: true,
  },
  projects: [
    {
      name: "setup",
      testMatch: /auth\.setup\.ts/,
    },
    {
      name: "chromium",
      dependencies: ["setup"],
      use: {
        browserName: "chromium",
        storageState: storageStatePath,
      },
    },
  ],
})
