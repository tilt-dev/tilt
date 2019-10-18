import React from "react"
import { storiesOf } from "@storybook/react"
import AnalyticsNudge from "./AnalyticsNudge"

storiesOf("AnalyticsNudge", module).add("needsNudge", () => (
  <AnalyticsNudge needsNudge={true} />
))
