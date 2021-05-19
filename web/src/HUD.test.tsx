import { mount } from "enzyme"
import { createMemoryHistory } from "history"
import React from "react"
import ReactDOM from "react-dom"
import ReactModal from "react-modal"
import { MemoryRouter } from "react-router"
import HUD from "./HUD"
import SocketBar from "./SocketBar"
import { oneResourceView } from "./testdata"
import { SocketState } from "./types"

// Note: `body` is used as the app element _only_ in a test env
// since the app root element isn't available; in prod, it should
// be set as the app root so that accessibility features are set correctly
ReactModal.setAppElement(document.body)

declare global {
  namespace NodeJS {
    interface Global {
      document: Document
      window: Window
      navigator: Navigator
    }
  }
}

const fakeHistory = createMemoryHistory()
const interfaceVersion = { isNewDefault: () => false, toggleDefault: () => {} }
const emptyHUD = () => {
  return (
    <MemoryRouter initialEntries={["/"]}>
      <HUD history={fakeHistory} interfaceVersion={interfaceVersion} />
    </MemoryRouter>
  )
}
const HUDAtPath = (path: string) => {
  return (
    <MemoryRouter initialEntries={[path]}>
      <HUD history={fakeHistory} interfaceVersion={interfaceVersion} />
    </MemoryRouter>
  )
}

beforeEach(() => {
  Date.now = jest.fn(() => 1482363367071)
})

it("renders without crashing", () => {
  const div = document.createElement("div")
  ReactDOM.render(emptyHUD(), div)
  ReactDOM.unmountComponentAtNode(div)
})

it("renders reconnecting bar", async () => {
  const root = mount(emptyHUD())
  const hud = root.find(HUD)
  expect(hud.text()).toEqual(expect.stringContaining("Loading"))

  hud.setState({
    view: oneResourceView(),
    socketState: SocketState.Reconnecting,
  })

  let socketBar = root.find(SocketBar)
  expect(socketBar).toHaveLength(1)
  expect(socketBar.at(0).text()).toEqual(
    expect.stringContaining("Reconnecting")
  )
})

it("loads logs incrementally", async () => {
  const root = mount(emptyHUD())
  const hud = root.find(HUD).instance() as HUD

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
  hud.setAppState({ view: resourceView })

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
  hud.setAppState({ view: resourceView2 })

  root.update()
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
  const root = mount(emptyHUD())
  const hud = root.find(HUD).instance() as HUD

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
  hud.setAppState({ view: resourceView })

  root.update()
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
