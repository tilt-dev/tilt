import React from "react"
import ReactDOM from "react-dom"
import { storiesOf } from "@storybook/react"
import LogPane from "./LogPane"
import { LogLine } from "./types"
import { MemoryRouter } from "react-router"

function logPane(lines: LogLine[], options?: any) {
  let nullFn = function() {}
  return (
    <MemoryRouter>
      <LogPane
        manifestName={options?.manifestName}
        logLines={lines}
        showManifestPrefix={true}
        handleSetHighlight={nullFn}
        handleClearHighlight={nullFn}
        highlight={null}
        isSnapshot={false}
        traceNav={options?.traceNav}
      />
    </MemoryRouter>
  )
}

function line(manifestName: string, text: string, level?: string): LogLine {
  level = level || "INFO"
  return { manifestName, text, level, spanId: "" }
}

function initLine(manifestName: string, text: string): LogLine {
  return {
    manifestName,
    text,
    level: "INFO",
    spanId: "build:1",
    buildEvent: "init",
  }
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

function firstBuild() {
  let lines = [
    initLine("fe", "Initial build - fe"),
    line("fe", "Building fe [1/3]"),
    line("fe", "Building fe [2/3]"),
    line("fe", "Building fe [3/3]"),
    line("fe", "Pod fe [1/3]"),
    line("fe", "Pod fe [2/3]"),
    line("fe", "Pod fe [3/3]"),
  ]
  let traceNav = {
    current: { url: "/", label: "Build #1" },
    next: { url: "/", label: "Build #2" },
  }
  return logPane(lines, { traceNav: traceNav, manifestName: "fe" })
}

function lastBuild() {
  let lines = [
    initLine("fe", "Initial build - fe"),
    line("fe", "Building fe [1/3]"),
    line("fe", "Building fe [2/3]"),
    line("fe", "Building fe [3/3]"),
    line("fe", "Pod fe [1/3]"),
    line("fe", "Pod fe [2/3]"),
    line("fe", "Pod fe [3/3]"),
  ]
  let traceNav = {
    current: { url: "/", label: "Build #2" },
    prev: { url: "/", label: "Build #1" },
  }
  return logPane(lines, { traceNav: traceNav, manifestName: "fe" })
}

storiesOf("LogPane", module)
  .add("three-resources", threeResources)
  .add("first-build", firstBuild)
  .add("last-build", lastBuild)
