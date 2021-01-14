import React from "react"
import MetricsDialog from "./MetricsDialog"

export default {
  title: "MetricsDialog",
  argTypes: { onClose: { action: "closed" } },
}

export let Teaser = (args: any) => {
  let serving = { mode: "", grafanaHost: "" }
  return (
    <MetricsDialog
      open={true}
      serving={serving}
      anchorEl={document.body}
      onClose={args.onClose}
    />
  )
}

export let Loading = (args: any) => {
  let serving = { mode: "local" }
  return (
    <MetricsDialog
      open={true}
      serving={serving}
      anchorEl={document.body}
      onClose={args.onClose}
    />
  )
}

export let Graphs = (args: any) => {
  let serving = { mode: "local", grafanaHost: "localhost:10352" }
  return (
    <MetricsDialog
      open={true}
      serving={serving}
      anchorEl={document.body}
      onClose={args.onClose}
    />
  )
}
