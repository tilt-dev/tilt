import { storiesOf } from "@storybook/react"
import React from "react"
import AnalyticsNudge from "./AnalyticsNudge"

storiesOf("AnalyticsNudge", module).add("needsNudge", () => (
  <AnalyticsNudge needsNudge={true} />
))
