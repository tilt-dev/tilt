import React, { Component } from "react"
import { findRenderedComponentWithType } from "react-dom/test-utils"
import ReactModal from "react-modal"
import { useNavigate, useLocation } from "react-router-dom"
import { MemoryRouter } from "react-router"
import HUD, { mergeAppUpdate } from "./HUD"
import LogStore from "./LogStore"
import { SocketBarRoot } from "./SocketBar"
import { renderTestComponent } from "./test-helpers"
import {
  logList,
  nButtonView,
  nResourceView,
  oneResourceView,
  twoResourceView,
} from "./testdata"
import { SocketState } from "./types"

// Note: `body` is used as the app element _only_ in a test env
// since the app root element isn't available; in prod, it should
// be set as the app root so that accessibility features are set correctly
ReactModal.setAppElement(document.body)

const interfaceVersion = { isNewDefault: () => false, toggleDefault: () => {} }

let InjectHUD = () => {
  let navigate = useNavigate()
  let location = useLocation()
  return (
    <HUD
      navigate={navigate}
      location={location}
      interfaceVersion={interfaceVersion}
    />
  )
}

class RouterHUD extends Component {
  render() {
    return (
      <MemoryRouter
        initialEntries={["/"]}
        future={{ v7_startTransition: true, v7_relativeSplatPath: true }}
      >
        <InjectHUD />
      </MemoryRouter>
    )
  }
}

beforeEach(() => {
  Date.now = jest.fn(() => 1482363367071)
})

it("renders reconnecting bar", async () => {
  const { rootTree, container } = renderTestComponent(<RouterHUD />)
  expect(container.textContent).toEqual(expect.stringContaining("Loading"))

  const hud = findRenderedComponentWithType(rootTree, HUD)

  hud.setState({
    view: oneResourceView(),
    socketState: SocketState.Reconnecting,
  })

  let socketBar = Array.from(container.querySelectorAll(SocketBarRoot))
  expect(socketBar).toHaveLength(1)
  expect(socketBar[0].textContent).toEqual(
    expect.stringContaining("reconnecting")
  )
})

it("loads logs incrementally", async () => {
  const { rootTree } = renderTestComponent(<RouterHUD />)
  const hud = findRenderedComponentWithType(rootTree, HUD)

  let now = new Date().toString()
  let resourceView = oneResourceView()
  resourceView.logList = {
    spans: {
      "": {},
    },
    segments: [
      { text: "line1\n", time: now },
      { text: "line2\n", time: now },
    ],
    fromCheckpoint: 0,
    toCheckpoint: 2,
  }
  hud.onAppChange({ view: resourceView })

  let resourceView2 = oneResourceView()
  resourceView2.logList = {
    spans: {
      "": {},
    },
    segments: [
      { text: "line3\n", time: now },
      { text: "line4\n", time: now },
    ],
    fromCheckpoint: 2,
    toCheckpoint: 4,
  }
  hud.onAppChange({ view: resourceView2 })

  let snapshot = hud.snapshotFromState(hud.state)
  expect(snapshot.view?.logList).toEqual({
    spans: {
      _: { manifestName: "" },
    },
    segments: [
      { text: "line1\n", time: now, spanId: "_" },
      { text: "line2\n", time: now, spanId: "_" },
      { text: "line3\n", time: now, spanId: "_" },
      { text: "line4\n", time: now, spanId: "_" },
    ],
  })
})

it("renders logs to snapshot", async () => {
  const { rootTree } = renderTestComponent(<RouterHUD />)
  const hud = findRenderedComponentWithType(rootTree, HUD)

  let now = new Date().toString()
  let resourceView = oneResourceView()
  resourceView.logList = {
    spans: {
      "": {},
    },
    segments: [
      { text: "line1\n", time: now, level: "WARN" },
      { text: "line2\n", time: now, fields: { buildEvent: "1" } },
    ],
    fromCheckpoint: 0,
    toCheckpoint: 2,
  }
  hud.onAppChange({ view: resourceView })

  let snapshot = hud.snapshotFromState(hud.state)
  expect(snapshot.view?.logList).toEqual({
    spans: {
      _: { manifestName: "" },
    },
    segments: [
      { text: "line1\n", time: now, spanId: "_", level: "WARN" },
      { text: "line2\n", time: now, spanId: "_", fields: { buildEvent: "1" } },
    ],
  })
})

describe("mergeAppUpdates", () => {
  // It's important to maintain reference equality when nothing changes.
  it("handles no view update", () => {
    let resourceView = oneResourceView()
    let prevState = { view: resourceView }
    let result = mergeAppUpdate(prevState as any, {}) as any
    expect(result).toBe(null)
  })

  it("handles empty view update", () => {
    let resourceView = oneResourceView()
    let prevState = { view: resourceView }
    let result = mergeAppUpdate(prevState as any, { view: {} })
    expect(result).toBe(null)
  })

  it("handles replace view update", () => {
    let prevState = { view: oneResourceView() }
    let update = { view: oneResourceView() }
    let result = mergeAppUpdate(prevState as any, update)
    expect(result!.view).not.toBe(update.view)
    expect(result!.view).not.toBe(prevState.view)
    expect(result!.view.uiSession).toBe(update.view.uiSession)
  })

  it("handles add resource", () => {
    let prevState = { view: oneResourceView() }
    let update = { view: { uiResources: [twoResourceView().uiResources[1]] } }
    let result = mergeAppUpdate(prevState as any, update)
    expect(result!.view).not.toBe(prevState.view)
    expect(result!.view.uiSession).toBe(prevState.view.uiSession)
    expect(result!.view.uiResources!.length).toEqual(2)
    expect(result!.view.uiResources![0].metadata!.name).toEqual("vigoda")
    expect(result!.view.uiResources![1].metadata!.name).toEqual("snack")
  })

  it("handles add resource out of order", () => {
    let prevState = { view: nResourceView(10) }
    let addedResources = prevState.view.uiResources.splice(3, 1)

    let update = { view: { uiResources: addedResources } }
    let result = mergeAppUpdate(prevState as any, update)
    expect(result!.view).not.toBe(prevState.view)
    expect(result!.view.uiSession).toBe(prevState.view.uiSession)
    expect(result!.view.uiResources).toEqual(nResourceView(10).uiResources)
  })

  it("handles add button out of order", () => {
    let prevState = { view: nButtonView(9) }
    let addedButtons = prevState.view.uiButtons.splice(3, 1)

    let update = { view: { uiButtons: addedButtons } }
    let result = mergeAppUpdate(prevState as any, update)
    expect(result!.view).not.toBe(prevState.view)
    expect(result!.view.uiSession).toBe(prevState.view.uiSession)
    expect(result!.view.uiButtons).toEqual(nButtonView(9).uiButtons)
  })

  it("handles delete resource", () => {
    let prevState = { view: twoResourceView() }
    let update = {
      view: {
        uiResources: [
          {
            metadata: {
              name: "vigoda",
              deletionTimestamp: new Date().toString(),
            },
          },
        ],
      },
    }
    let result = mergeAppUpdate(prevState as any, update)
    expect(result!.view).not.toBe(prevState.view)
    expect(result!.view.uiResources!.length).toEqual(1)
    expect(result!.view.uiResources![0].metadata!.name).toEqual("snack")
  })

  it("handles replace resource", () => {
    let prevState = { view: twoResourceView() }
    let update = { view: { uiResources: [{ metadata: { name: "vigoda" } }] } }
    let result = mergeAppUpdate(prevState as any, update)
    expect(result!.view).not.toBe(prevState.view)
    expect(result!.view.uiResources!.length).toEqual(2)
    expect(result!.view.uiResources![0]).toBe(update.view.uiResources[0])
    expect(result!.view.uiResources![1]).toBe(prevState.view.uiResources[1])
  })

  it("handles add button", () => {
    let prevState = { view: nButtonView(1) }
    let update = { view: { uiButtons: [nButtonView(2).uiButtons[1]] } }
    let result = mergeAppUpdate(prevState as any, update)
    expect(result!.view).not.toBe(prevState.view)
    expect(result!.view.uiSession).toBe(prevState.view.uiSession)
    expect(result!.view.uiResources).toBe(prevState.view.uiResources)
    expect(result!.view.uiButtons!.length).toEqual(2)
    expect(result!.view.uiButtons![0].metadata!.name).toEqual("button1")
    expect(result!.view.uiButtons![1].metadata!.name).toEqual("button2")
  })

  it("handles delete button", () => {
    let prevState = { view: nButtonView(2) }
    let update = {
      view: {
        uiButtons: [
          {
            metadata: {
              name: "button1",
              deletionTimestamp: new Date().toString(),
            },
          },
        ],
      },
    }
    let result = mergeAppUpdate(prevState as any, update)
    expect(result!.view).not.toBe(prevState.view)
    expect(result!.view.uiResources).toBe(prevState.view.uiResources)
    expect(result!.view.uiButtons!.length).toEqual(1)
    expect(result!.view.uiButtons![0].metadata!.name).toEqual("button2")
  })

  it("handles replace button", () => {
    let prevState = { view: nButtonView(2) }
    let update = { view: { uiButtons: [{ metadata: { name: "button1" } }] } }
    let result = mergeAppUpdate(prevState as any, update)
    expect(result!.view).not.toBe(prevState.view)
    expect(result!.view.uiResources).toBe(prevState.view.uiResources)
    expect(result!.view.uiButtons!.length).toEqual(2)
    expect(result!.view.uiButtons![0]).toBe(update.view.uiButtons[0])
    expect(result!.view.uiButtons![1]).toBe(prevState.view.uiButtons[1])
  })

  it("handles socket state", () => {
    let prevState = { view: twoResourceView(), socketState: SocketState.Active }
    let update = { socketState: SocketState.Reconnecting }
    let result = mergeAppUpdate(prevState as any, update) as any
    expect(result!.view).toBe(prevState.view)
    expect(result!.socketState).toBe(SocketState.Reconnecting)
  })

  it("handles complete view", () => {
    let prevLogStore = new LogStore()
    let prevState = { view: twoResourceView(), logStore: prevLogStore }

    let update = {
      view: {
        uiResources: [{ metadata: { name: "b" } }, { metadata: { name: "a" } }],
        uiButtons: [{ metadata: { name: "z" } }, { metadata: { name: "y" } }],
        logList: logList(["line1", "line2"]),
        isComplete: true,
      },
    }
    let result = mergeAppUpdate<"view" | "logStore">(prevState as any, update)
    expect(result!.view).toBe(update.view)
    expect(result!.logStore).not.toBe(prevState.logStore)
    expect(result!.logStore?.allLog().map((ll) => ll.text)).toEqual([
      "line1",
      "line2",
    ])
    const expectedResourceOrder = ["a", "b"]
    expect(result!.view.uiResources?.map((r) => r.metadata!.name)).toEqual(
      expectedResourceOrder
    )
    const expectedButtonOrder = ["y", "z"]
    expect(result!.view.uiButtons?.map((r) => r.metadata!.name)).toEqual(
      expectedButtonOrder
    )
  })

  it("handles log only update", () => {
    let prevLogStore = new LogStore()
    let prevState = { view: twoResourceView(), logStore: prevLogStore }

    let update = {
      view: {
        logList: logList(["line1", "line2"]),
      },
    }
    let result = mergeAppUpdate<"view" | "logStore">(prevState as any, update)
    expect(result).toBe(null)
  })
})
