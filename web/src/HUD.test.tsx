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
} from "./testdata.test"
import { createMemoryHistory } from "history"
import { SocketState } from "./types"

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
    View: oneResourceView(),
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
  hud.setState({ View: oneResourceView() })
  expect(root.find(".Statusbar")).toHaveLength(1)
  expect(root.find(".Sidebar")).toHaveLength(1)
})

it("opens sidebar on click", async () => {
  const root = mount(emptyHUD())
  const hud = root.find(HUD)
  hud.setState({ View: oneResourceView() })

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
  hud.setState({ View: resourceView })
  let oldDOMNode = root.find(".Sidebar").getDOMNode()
  let resource = resourceView.Resources[0]
  if (!resource.K8sResourceInfo) throw new Error("missing k8s info")
  resource.K8sResourceInfo.PodLog += "hello world\n"
  hud.setState({ View: resourceView })
  let newDOMNode = root.find(".Sidebar").getDOMNode()

  expect(oldDOMNode).toBe(newDOMNode)
})

it("does re-render the sidebar when the resource list changes", async () => {
  const root = mount(emptyHUD())
  const hud = root.find(HUD)

  let resourceView = oneResourceView()
  hud.setState({ View: resourceView })
  let sidebarLinks = root.find(".Sidebar-resources Link")
  expect(sidebarLinks).toHaveLength(2)

  let newResourceView = twoResourceView()
  hud.setState({ View: newResourceView })
  sidebarLinks = root.find(".Sidebar-resources Link")
  expect(sidebarLinks).toHaveLength(3)
})

it("renders tab nav", () => {
  const root = mount(emptyHUD())
  const hud = root.find(HUD)

  let resourceView = oneResourceView()
  hud.setState({ View: resourceView })
  let tabNavLinks = root.find(".TabNav Link")
  expect(tabNavLinks).toHaveLength(2)
})

it("renders number of errors in tabnav when no resource is selected", () => {
  const root = mount(emptyHUD())
  const hud = root.find(HUD)

  let resourceView = twoResourceView()
  hud.setState({ View: resourceView })
  let errorTab = root.find(".tabLink--errors")
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
  hud.setState({ View: resourceView })
  let errorTab = root.find(".tabLink--errors")
  expect(errorTab.at(0).text()).toEqual("Alerts1")
})

it("renders two errors for a resource that has pod restarts and a build failure", () => {
  const root = mount(emptyHUD())
  const hud = root.find(HUD)

  let resourceView = oneResourceView()
  let resource = resourceView.Resources[0]
  if (!resource.K8sResourceInfo) throw new Error("missing k8s info")
  resource.K8sResourceInfo.PodRestarts = 1
  hud.setState({ View: resourceView })
  let errorTab = root.find(".tabLink--errors")
  expect(errorTab.at(0).text()).toEqual("Alerts2")
})

it("renders two errors for a resource that has pod restarts, a build failure and is in the error state", () => {
  const root = mount(emptyHUD())
  const hud = root.find(HUD)

  let resourceView = oneResourceView()
  let resource = resourceView.Resources[0]
  if (!resource.K8sResourceInfo) throw new Error("missing k8s info")
  resource.K8sResourceInfo.PodRestarts = 1
  resourceView.Resources[0].RuntimeStatus = "CrashLoopBackOff"
  hud.setState({ View: resourceView })
  let errorTab = root.find(".tabLink--errors")
  expect(errorTab.at(0).text()).toEqual("Alerts2")
})

it("renders no error count in tabnav if there are no errors", () => {
  const root = mount(emptyHUD())
  const hud = root.find(HUD)

  hud.setState({ View: { Resources: [oneResourceNoAlerts()] } })
  let errorTab = root.find(".tabLink--errors")
  expect(errorTab.at(0).text()).toEqual("Alerts")
})

it("log page for nonexistent resource shows error", async () => {
  const root = mount(HUDAtPath("/r/nonexistentresource"))
  const hud = root.find(HUD)
  hud.setState({ View: oneResourceView() })

  let loadingScreen = root.find(".HeroScreen")
  expect(loadingScreen.at(0).text()).toEqual(
    "No resource found at /r/nonexistentresource"
  )
})

it("alerts page for nonexistent resource shows error", async () => {
  const root = mount(HUDAtPath("/r/nonexistentresource/alerts"))
  const hud = root.find(HUD)
  hud.setState({ View: oneResourceView() })

  let loadingScreen = root.find(".HeroScreen")
  expect(loadingScreen.at(0).text()).toEqual(
    "No resource found at /r/nonexistentresource/alerts"
  )
})

it("renders snapshot button if snapshots are enabled and this isn't a snapshot view", async () => {
  const root = mount(HUDAtPath("/"))
  const hud = root.find(HUD)
  let view = oneResourceView()
  view.FeatureFlags = { snapshots: true }
  hud.setState({ View: view })

  let snapshotSection = root.find(".TopBar-snapshotButton")
  expect(snapshotSection.exists()).toBe(true)
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
  view.FeatureFlags = { snapshots: true }
  hud.setState({ View: view })

  let snapshotSection = root.find(".TopBar-snapshotButton")
  expect(snapshotSection.exists()).toBe(false)
})
