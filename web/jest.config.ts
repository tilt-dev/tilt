import { Config } from "@jest/types"

const config: Config.InitialOptions = {
  setupFilesAfterEnv: ["<rootDir>/setupTests.ts"]
}

export default config
