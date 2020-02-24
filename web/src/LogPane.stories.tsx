import React from "react"
import ReactDOM from "react-dom"
import { storiesOf } from "@storybook/react"
import LogPane from "./LogPane"
import { LogLine } from "./types"

function logPane(lines: LogLine[]) {
  let nullFn = function() {}
  return (
    <LogPane
      manifestName=""
      logLines={lines}
      showManifestPrefix={true}
      handleSetHighlight={nullFn}
      handleClearHighlight={nullFn}
      highlight={null}
      isSnapshot={false}
    />
  )
}

function line(manifestName: string, text: string, level?: string): LogLine {
  level = level || "INFO"
  return { manifestName, text, level, spanId: manifestName }
}

function threeResources() {
  let lines = [
    line("fe", "Building fe [1/3]"),
    line("fe", "Building fe [2/3]"),
    line("fe", "Building fe [3/3]"),
    line("letters", "Building letters [1/3]"),
    line("letters", "Building letters [2/3]"),
    line("letters", "Building letters [3/3]"),
    line("fe", "Pod fe [1/3]"),
    line("fe", "Pod fe [2/3]"),
    line("fe", "Pod fe [3/3]"),
    line("numbers", "Building numbers [1/3]"),
    line("letters", "Pod letters rollout warning", "WARN"),
    line("letters", "Pod letters [1/3]"),
    line("letters", "Pod letters [2/3]"),
    line("letters", "Pod letters [3/3]"),
    line("numbers", "Building numbers [2/3]"),
    line("numbers", "Building numbers [3/3]"),
    line("numbers", "Pod numbers [1/3]"),
    line("numbers", "Pod numbers [2/3]"),
    line("numbers", "Pod numbers [3/3]"),
  ]
  return logPane(lines)
}

storiesOf("LogPane", module).add("three-resources", threeResources)
