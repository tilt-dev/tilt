import { configure } from "@storybook/react"
import "../src/index.scss"

// automatically import all files ending in *.stories.tsx
configure(require.context("../src", true, /\.stories\.tsx$/), module)
