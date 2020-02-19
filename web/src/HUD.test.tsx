import React from "react"
import ReactDOM from "react-dom"
import { MemoryRouter } from "react-router"
import HUD from "./HUD"
import SocketBar from "./SocketBar"
import { mount } from "enzyme"
import {
  oneResourceView,
  twoResourceView,
  oneResourceNoAlerts,
} from "./testdata"
import { createMemoryHistory } from "history"
import { SocketState } from "./types"
import ReactModal from "react-modal"

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
const emptyHUD = () => {
  return (
    <MemoryRouter initialEntries={["/"]}>
      <HUD history={fakeHistory} />
    </MemoryRouter>
  )
}
const HUDAtPath = (path: string) => {
  return (
    <MemoryRouter initialEntries={[path]}>
      <HUD history={fakeHistory} />
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

it("renders resource", async () => {
  const root = mount(emptyHUD())
  const hud = root.find(HUD)
  hud.setState({ view: oneResourceView() })
  expect(root.find(".Statusbar")).toHaveLength(1)
  expect(root.find(".Sidebar")).toHaveLength(1)
})

it("opens sidebar on click", async () => {
  const root = mount(emptyHUD())
  const hud = root.find(HUD)
  hud.setState({ view: oneResourceView() })

  let sidebar = root.find(".Sidebar")
  expect(sidebar).toHaveLength(1)
  expect(sidebar.hasClass("is-closed")).toBe(false)

  let button = root.find("button.Sidebar-toggle")
  expect(button).toHaveLength(1)
  button.simulate("click")

  sidebar = root.find(".Sidebar")
  expect(sidebar).toHaveLength(1)
  expect(sidebar.hasClass("is-closed")).toBe(true)
})

it("doesn't re-render the sidebar when the logs change", async () => {
  const root = mount(emptyHUD())
  const hud = root.find(HUD)

  let resourceView = oneResourceView()
  hud.setState({ view: resourceView })
  let oldDOMNode = root.find(".Sidebar").getDOMNode()

  function now() {
    return new Date().toString()
  }
  resourceView.logList = {
    spans: {
      "": {},
    },
    segments: [
      { text: "line1\n", time: now() },
      { text: "line2\n", time: now() },
    ],
  }
  hud.setState({ view: resourceView })

  let newDOMNode = root.find(".Sidebar").getDOMNode()
  expect(oldDOMNode).toBe(newDOMNode)
})

it("does re-render the sidebar when the resource list changes", async () => {
  const root = mount(emptyHUD())
  const hud = root.find(HUD)

  let resourceView = oneResourceView()
  hud.setState({ view: resourceView })
  let sidebarLinks = root.find(".Sidebar-resources Link")
  expect(sidebarLinks).toHaveLength(2)

  let newResourceView = twoResourceView()
  hud.setState({ view: newResourceView })
  sidebarLinks = root.find(".Sidebar-resources Link")
  expect(sidebarLinks).toHaveLength(3)
})

it("renders tab nav", () => {
  const root = mount(emptyHUD())
  const hud = root.find(HUD)

  let resourceView = oneResourceView()
  hud.setState({ view: resourceView })
  let tabNavLinks = root.find(".secondaryNav Link")
  expect(tabNavLinks).toHaveLength(2)
})

it("renders number of errors in tabnav when no resource is selected", () => {
  const root = mount(emptyHUD())
  const hud = root.find(HUD)

  let resourceView = twoResourceView()
  hud.setState({ view: resourceView })
  let errorTab = root.find(".secondaryNavLink--alerts")
  expect(errorTab.at(0).text()).toEqual("Alerts2")
})

it("renders the number of errors a resource has in tabnav when a resource is selected", () => {
  const root = mount(
    <MemoryRouter initialEntries={["/r/vigoda"]}>
      <HUD history={fakeHistory} />
    </MemoryRouter>
  )
  const hud = root.find(HUD)

  let resourceView = twoResourceView()
  hud.setState({ view: resourceView })
  let errorTab = root.find(".secondaryNavLink--alerts")
  expect(errorTab.at(0).text()).toEqual("Alerts1")
})

it("renders two errors for a resource that has pod restarts and a build failure", () => {
  const root = mount(emptyHUD())
  const hud = root.find(HUD)

  let resourceView = oneResourceView()
  let resource = resourceView.resources[0]
  if (!resource.k8sResourceInfo) throw new Error("missing k8s info")
  resource.k8sResourceInfo.podRestarts = 1
  hud.setState({ view: resourceView })
  let errorTab = root.find(".secondaryNavLink--alerts")
  expect(errorTab.at(0).text()).toEqual("Alerts2")
})

it("renders two errors for a resource that has pod restarts, a build failure and is in the error state", () => {
  const root = mount(emptyHUD())
  const hud = root.find(HUD)

  let resourceView = oneResourceView()
  let resource = resourceView.resources[0]
  if (!resource.k8sResourceInfo) throw new Error("missing k8s info")
  resource.k8sResourceInfo.podRestarts = 1
  resourceView.resources[0].runtimeStatus = "CrashLoopBackOff"
  hud.setState({ view: resourceView })
  let errorTab = root.find(".secondaryNavLink--alerts")
  expect(errorTab.at(0).text()).toEqual("Alerts2")
})

it("renders no error count in tabnav if there are no errors", () => {
  const root = mount(emptyHUD())
  const hud = root.find(HUD)

  hud.setState({ view: { resources: [oneResourceNoAlerts()] } })
  let errorTab = root.find(".secondaryNavLink--alerts")
  expect(errorTab.at(0).text()).toEqual("Alerts")
})

it("log page for nonexistent resource shows error", async () => {
  const root = mount(HUDAtPath("/r/nonexistentresource"))
  const hud = root.find(HUD)
  hud.setState({ view: oneResourceView() })

  let loadingScreen = root.find(".HeroScreen")
  expect(loadingScreen.at(0).text()).toEqual(
    "No resource found at /r/nonexistentresource"
  )
})

it("alerts page for nonexistent resource shows error", async () => {
  const root = mount(HUDAtPath("/r/nonexistentresource/alerts"))
  const hud = root.find(HUD)
  hud.setState({ view: oneResourceView() })

  let loadingScreen = root.find(".HeroScreen")
  expect(loadingScreen.at(0).text()).toEqual(
    "No resource found at /r/nonexistentresource/alerts"
  )
})

it("renders snapshot button if snapshots are enabled and this isn't a snapshot view", async () => {
  const root = mount(HUDAtPath("/"))
  const hud = root.find(HUD)
  let view = oneResourceView()
  view.featureFlags = { snapshots: true }
  hud.setState({ view: view })

  let button = root.find("button.snapshotButton")
  expect(button.exists()).toBe(true)

  button.simulate("click")
  root.update()
  expect(hud.state().showSnapshotModal).toBe(true)
})

it("doesn't render snapshot button if snapshots are enabled and this is a snapshot view", async () => {
  global.window = Object.create(window)
  Object.defineProperty(window, "location", {
    value: {
      host: "localhost",
      pathname: "/snapshot/aaaaaa",
    },
  })

  const root = mount(HUDAtPath("/"))
  const hud = root.find(HUD)

  let view = oneResourceView()
  view.featureFlags = { snapshots: true }
  hud.setState({ view: view })

  let snapshotSection = root.find(".TopBar-toolsButton")
  expect(snapshotSection.exists()).toBe(false)
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
