import { storiesOf } from "@storybook/react"
import React from "react"
import HeroScreen from "./HeroScreen"

storiesOf("HeroScreen", module).add("loading", () => (
  <HeroScreen message={"Loading…"} />
))
