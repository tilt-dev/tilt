import React from "react"
import { storiesOf } from "@storybook/react"
import LogPaneLine from "./LogPaneLine"
import { MemoryRouter } from "react-router"

function infoLine() {
  return (
    <div className="LogPane">
      <LogPaneLine
        text="Hello world"
        manifestName="fe"
        level="INFO"
        lineId={1}
        shouldHighlight={false}
        showManifestPrefix={true}
        isContextChange={false}
      />
    </div>
  )
}

function warnLine() {
  return (
    <div className="LogPane">
      <LogPaneLine
        text="Hello world"
        manifestName="fe"
        level="WARN"
        lineId={1}
        shouldHighlight={false}
        showManifestPrefix={true}
        isContextChange={false}
      />
    </div>
  )
}

function threeLines() {
  return (
    <div className="LogPane">
      <LogPaneLine
        text="line 1"
        manifestName="fe"
        level="INFO"
        lineId={1}
        shouldHighlight={false}
        showManifestPrefix={true}
        isContextChange={false}
      />
      <LogPaneLine
        text="line 2"
        manifestName="fe"
        level="INFO"
        lineId={1}
        shouldHighlight={false}
        showManifestPrefix={true}
        isContextChange={false}
      />
      <LogPaneLine
        text="line 3"
        manifestName="fe"
        level="INFO"
        lineId={1}
        shouldHighlight={false}
        showManifestPrefix={true}
        isContextChange={false}
      />
    </div>
  )
}

function buildEventInit() {
  return (
    <MemoryRouter>
      <div className="LogPane">
        <LogPaneLine
          text="Initial build - fe"
          manifestName="fe"
          level="INFO"
          lineId={1}
          shouldHighlight={false}
          showManifestPrefix={true}
          isContextChange={false}
          buildEvent={"init"}
          traceUrl={"/"}
        />
      </div>
    </MemoryRouter>
  )
}

function buildEventFallback() {
  return (
    <div className="LogPane" style={{ marginTop: "100px" }}>
      <LogPaneLine
        text="Falling back"
        manifestName="fe"
        level="INFO"
        lineId={1}
        shouldHighlight={false}
        showManifestPrefix={true}
        isContextChange={false}
        buildEvent={"fallback"}
      />
      <LogPaneLine
        text="Falling back line 2"
        manifestName="fe"
        level="INFO"
        lineId={1}
        shouldHighlight={false}
        showManifestPrefix={true}
        isContextChange={false}
        buildEvent={"fallback"}
      />
    </div>
  )
}

storiesOf("LogPaneLine", module)
  .add("infoLine", infoLine)
  .add("warnLine", warnLine)
  .add("threeLines", threeLines)
  .add("buildEventInit", buildEventInit)
  .add("buildEventFallback", buildEventFallback)
