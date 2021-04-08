import { fireEvent } from "@testing-library/dom"
import { mount } from "enzyme"
import React from "react"
import {
  cleanupMockAnalyticsCalls,
  expectIncrs,
  mockAnalyticsCalls,
} from "./analytics_test_helpers"
import { logLinesToString } from "./logs"
import LogStore from "./LogStore"
import OverviewActionBarKeyboardShortcuts from "./OverviewActionBarKeyboardShortcuts"
import { appendLinesForManifestAndSpan } from "./testlogs"

beforeEach(() => {
  mockAnalyticsCalls()
})
afterEach(() => {
  cleanupMockAnalyticsCalls()
})

function numKeyCode(num: number): number {
  return num + 48
}

type Link = Proto.webviewLink

let logStore: LogStore | null
let component: any
let endpointUrl = ""
const shortcuts = (endpoints: Link[]) => {
  logStore = new LogStore()
  endpointUrl = ""
  component = mount(
    <OverviewActionBarKeyboardShortcuts
      logStore={logStore}
      resourceName={"fake-resource"}
      endpoints={endpoints}
      openEndpointUrl={(url) => (endpointUrl = url)}
    />
  )
}

afterEach(() => {
  if (component) {
    component.unmount()
    component = null
  }
  if (logStore) {
    logStore = null
  }
})

describe("endpoints", () => {
  it("zero endpoint urls", () => {
    shortcuts([])
    fireEvent.keyDown(document.body, { keyCode: numKeyCode(1), shiftKey: true })
    expect(endpointUrl).toEqual("")
  })
  it("two endpoint urls trigger first", () => {
    shortcuts([
      { url: "https://tilt.dev:4000" },
      { url: "https://tilt.dev:4001" },
    ])
    fireEvent.keyDown(document.body, { keyCode: numKeyCode(1), shiftKey: true })
    expect(endpointUrl).toEqual("https://tilt.dev:4000")
  })
  it("two endpoint urls trigger second", () => {
    shortcuts([
      { url: "https://tilt.dev:4000" },
      { url: "https://tilt.dev:4001" },
    ])
    fireEvent.keyDown(document.body, { keyCode: numKeyCode(2), shiftKey: true })
    expect(endpointUrl).toEqual("https://tilt.dev:4001")
  })
})

describe("clears logs", () => {
  it("meta key", () => {
    shortcuts([])
    appendLinesForManifestAndSpan(logStore!, "fake-resource", "span:1", [
      "line 1\n",
    ])
    fireEvent.keyDown(document.body, { key: "Backspace", metaKey: true })
    expect(logLinesToString(logStore!.allLog(), false)).toEqual("")
    expectIncrs({
      name: "ui.web.clearLogs",
      tags: { action: "shortcut", all: "false" },
    })
  })

  it("ctrl key", () => {
    shortcuts([])
    appendLinesForManifestAndSpan(logStore!, "fake-resource", "span:1", [
      "line 1\n",
    ])
    fireEvent.keyDown(document.body, { key: "Backspace", ctrlKey: true })
    expect(logLinesToString(logStore!.allLog(), false)).toEqual("")
    expectIncrs({
      name: "ui.web.clearLogs",
      tags: { action: "shortcut", all: "false" },
    })
  })
})
