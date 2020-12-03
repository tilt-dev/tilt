import React from "react"
import { storiesOf } from "@storybook/react"
import MetricsPane from "./MetricsPane"
import PathBuilder from "./PathBuilder"

let pb = PathBuilder.forTesting("localhost", "/")

function teaser() {
  let serving = { mode: "", grafanaHost: "" }
  return <MetricsPane pathBuilder={pb} serving={serving} />
}

function loading() {
  let serving = { mode: "" }
  return <MetricsPane pathBuilder={pb} serving={serving} />
}

function graphs() {
  let serving = { mode: "local", grafanaHost: "localhost:10352" }
  return <MetricsPane pathBuilder={pb} serving={serving} />
}

storiesOf("MetricsPane", module)
  .add("teaser", teaser)
  .add("graphs", graphs)
  .add("loading", loading)
