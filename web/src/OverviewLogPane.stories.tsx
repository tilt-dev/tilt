import React, { Component, useEffect, useState } from "react"
import { MemoryRouter } from "react-router"
import LogStore, { LogStoreProvider } from "./LogStore"
import OverviewLogPane from "./OverviewLogPane"

function now() {
  return new Date().toString()
}

type Line = string | { text: string; fields?: any }

function appendLines(logStore: LogStore, name: string, ...lines: Line[]) {
  let fromCheckpoint = logStore.checkpoint
  let toCheckpoint = fromCheckpoint + lines.length

  let spans = {} as any
  let spanId = name || "_"
  spans[spanId] = { manifestName: name }

  let segments = []
  for (let line of lines) {
    let obj = { time: now(), spanId: spanId, text: "" } as any
    if (typeof line == "string") {
      obj.text = line
    } else {
      for (let key in line) {
        obj[key] = (line as any)[key]
      }
    }
    segments.push(obj)
  }

  logStore.append({ spans, segments, fromCheckpoint, toCheckpoint })
}

export default {
  title: "OverviewLogPane",
  decorators: [
    (Story: any) => (
      <MemoryRouter initialEntries={["/"]}>
        <div
          style={{
            margin: "-1rem",
            height: "80vh",
            width: "80vw",
            border: "thin solid #ccc",
          }}
        >
          <Story />
        </div>
      </MemoryRouter>
    ),
  ],
}

export const ThreeLines = () => {
  let logStore = new LogStore()
  appendLines(logStore, "fe", "line 1\n", "line2\n", "line3\n")
  return (
    <LogStoreProvider value={logStore}>
      <OverviewLogPane manifestName="fe" />
    </LogStoreProvider>
  )
}

export const ThreeLinesAllLog = () => {
  let logStore = new LogStore()
  appendLines(logStore, "", "line 1\n", "line2\n", "line3\n")
  return (
    <LogStoreProvider value={logStore}>
      <OverviewLogPane manifestName="" />
    </LogStoreProvider>
  )
}

export const OneThousandLines = () => {
  let logStore = new LogStore()
  let lines = []
  for (let i = 0; i < 1000; i++) {
    lines.push(`line ${i}\n`)
  }
  appendLines(logStore, "fe", ...lines)
  return (
    <LogStoreProvider value={logStore}>
      <OverviewLogPane manifestName="fe" />
    </LogStoreProvider>
  )
}

export const StyledLines = () => {
  let logStore = new LogStore()
  let lines = [
    "Black: \u001b[30m text \u001b[0m\n",
    "Red: \u001b[31m text \u001b[0m\n",
    "Green: \u001b[32m text \u001b[0m\n",
    "Yellow: \u001b[33m text \u001b[0m\n",
    "Blue: \u001b[34m text \u001b[0m\n",
    "Magenta: \u001b[35m text \u001b[0m\n",
    "Cyan: \u001b[36m text \u001b[0m\n",
    "White: \u001b[37m text \u001b[0m\n",
    "Link: https://tilt.dev/\n",
    'Escaped Link: <a href="https://tilt.dev/" >Tilt</a>\n',
    'Escaped Button: <button onClick="alert(\\"you are p0wned\\")" >Tilt</button>\n',
  ]
  appendLines(logStore, "fe", ...lines)
  return (
    <LogStoreProvider value={logStore}>
      <OverviewLogPane manifestName="fe" />
    </LogStoreProvider>
  )
}

export const BuildEventLines = () => {
  let logStore = new LogStore()
  let lines = [
    { text: "Start build\n", fields: { buildEvent: "init" } },
    { text: "Fallback build\n", fields: { buildEvent: "fallback" } },
    "Build log 1\n",
    "Build log 2\n",
  ]
  appendLines(logStore, "fe", ...lines)
  return (
    <LogStoreProvider value={logStore}>
      <OverviewLogPane manifestName="fe" />
    </LogStoreProvider>
  )
}

export const ProgressLines = (args: any) => {
  let [logStore, setLogStore] = useState(new LogStore())
  let lines = [
    { text: "Start build\n", fields: { progressID: "start" } },
    { text: `Layer 1: 0%\n`, fields: { progressID: "layer1" } },
    { text: `Layer 2: 0%\n`, fields: { progressID: "layer2" } },
    { text: `Layer 3: 0%\n`, fields: { progressID: "layer3" } },
    { text: `Layer 4: 0%\n`, fields: { progressID: "layer4" } },
  ]
  appendLines(logStore, "fe", ...lines)

  useEffect(() => {
    let lines = [
      { text: "Start build\n", fields: { progressID: "start" } },
      { text: `Layer 1: ${args.layer1}%\n`, fields: { progressID: "layer1" } },
      { text: `Layer 2: ${args.layer2}%\n`, fields: { progressID: "layer2" } },
      { text: `Layer 3: ${args.layer3}%\n`, fields: { progressID: "layer3" } },
      { text: `Layer 4: ${args.layer4}%\n`, fields: { progressID: "layer4" } },
    ]
    appendLines(logStore, "fe", ...lines)
  }, [args])

  return (
    <LogStoreProvider value={logStore}>
      <OverviewLogPane manifestName="fe" />
    </LogStoreProvider>
  )
}

ProgressLines.args = {
  layer1: 50,
  layer2: 40,
  layer3: 30,
  layer4: 20,
}
ProgressLines.argTypes = {
  layer1: { control: { type: "number", min: 0, max: 100 } },
  layer2: { control: { type: "number", min: 0, max: 100 } },
  layer3: { control: { type: "number", min: 0, max: 100 } },
  layer4: { control: { type: "number", min: 0, max: 100 } },
}

class ForeverLogComponent extends Component {
  logStore = new LogStore()
  lineCount = 0
  timer: any

  componentDidMount() {
    this.timer = setInterval(() => {
      let lines = [
        { text: `Line #${this.lineCount++}\n` },
        { text: `Line #${this.lineCount++}\n` },
        { text: `Line #${this.lineCount++}\n` },
        { text: `Line #${this.lineCount++}\n` },
        { text: `Line #${this.lineCount++}\n` },
      ]
      appendLines(this.logStore, "fe", ...lines)
    }, 1000)
  }

  componentWillUnmount() {
    clearInterval(this.timer)
  }

  render() {
    return (
      <LogStoreProvider value={this.logStore}>
        <OverviewLogPane manifestName="fe" />
      </LogStoreProvider>
    )
  }
}

export const ForeverLog = () => {
  return <ForeverLogComponent />
}

export const BuildLogAndRunLog = (args: any) => {
  let logStore = new LogStore()
  let segments = []
  for (let i = 0; i < 10; i++) {
    segments.push({
      spanId: "build:1",
      text: `Vigoda build line ${i}\n`,
      time: new Date().toString(),
    })
  }
  for (let i = 0; i < 10; i++) {
    segments.push({
      spanId: "pod:1",
      text: `Vigoda pod line ${i}\n`,
      time: new Date().toString(),
    })
  }
  logStore.append({
    spans: {
      "build:1": { manifestName: "vigoda_1" },
      "pod:1": { manifestName: "vigoda_1" },
    },
    segments: segments,
  })

  return (
    <LogStoreProvider value={logStore}>
      <OverviewLogPane
        manifestName={"vigoda_1"}
        hideBuildLog={args.hideBuildLog}
        hideRunLog={args.hideRunLog}
      />
    </LogStoreProvider>
  )
}

BuildLogAndRunLog.args = {
  hideBuildLog: false,
  hideRunLog: false,
}

BuildLogAndRunLog.argTypes = {
  hideBuildLog: { control: { type: "boolean" } },
  hideRunLog: { control: { type: "boolean" } },
}
