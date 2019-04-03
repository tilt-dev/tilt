import React from "react"
import ReactDOM from "react-dom"
import HUD from "./HUD"
import { mount } from "enzyme"
import { RouteComponentProps } from "react-router-dom"
import { UnregisterCallback, Href } from "history"

// NOTE(dmiller) 4-02-19 this function is currently unused but I'm going to keep it around.
// I have a feeling that it will come in handy later.
function getMockRouterProps<P>(data: P) {
  var location = {
    hash: "",
    key: "",
    pathname: "",
    search: "",
    state: {},
  }

  var props: RouteComponentProps<P> = {
    match: {
      isExact: true,
      params: data,
      path: "",
      url: "",
    },
    location: location,
    history: {
      length: 2,
      action: "POP",
      location: location,
      push: () => {},
      replace: () => {},
      go: num => {},
      goBack: () => {},
      goForward: () => {},
      block: t => {
        var temp: UnregisterCallback = () => {}
        return temp
      },
      createHref: t => {
        var temp: Href = ""
        return temp
      },
      listen: t => {
        var temp: UnregisterCallback = () => {}
        return temp
      },
    },
    staticContext: {},
  }

  return props
}

const emptyHUD = () => {
  return <HUD />
}

it("renders without crashing", () => {
  const div = document.createElement("div")
  ReactDOM.render(emptyHUD(), div)
  ReactDOM.unmountComponentAtNode(div)
})

it("renders loading screen", async () => {
  const hud = mount(emptyHUD())
  expect(hud.html()).toEqual(expect.stringContaining("Loading"))

  hud.setState({ Message: "Disconnected" })
  expect(hud.html()).toEqual(expect.stringContaining("Disconnected"))
})

it("renders resource", async () => {
  const hud = mount(emptyHUD())
  hud.setState({ View: oneResourceView() })
  expect(hud.html())
  expect(hud.find(".Statusbar")).toHaveLength(1)
  expect(hud.find(".Sidebar")).toHaveLength(1)
})

it("opens sidebar on click", async () => {
  const hud = mount(emptyHUD())
  hud.setState({ View: oneResourceView() })

  let sidebar = hud.find(".Sidebar")
  expect(sidebar).toHaveLength(1)
  expect(sidebar.hasClass("is-closed")).toBe(false)

  let button = hud.find("button.Sidebar-toggle")
  expect(button).toHaveLength(1)
  button.simulate("click")

  sidebar = hud.find(".Sidebar")
  expect(sidebar).toHaveLength(1)
  expect(sidebar.hasClass("is-closed")).toBe(true)
})

it("doesn't re-render the sidebar when the logs change", async () => {
  const hud = mount(emptyHUD())
  let resourceView = oneResourceView()
  hud.setState({ View: resourceView })
  let oldDOMNode = hud.find(".Sidebar").getDOMNode()
  resourceView.Resources[0].PodLog += "hello world\n"
  hud.setState({ View: resourceView })
  let newDOMNode = hud.find(".Sidebar").getDOMNode()

  expect(oldDOMNode).toBe(newDOMNode)
})

it("does re-render the sidebar when the resource list changes", async () => {
  const hud = mount(emptyHUD())

  let resourceView = oneResourceView()
  hud.setState({ View: resourceView })
  let sidebarLinks = hud.find(".Sidebar Link")
  expect(sidebarLinks).toHaveLength(2)

  let newResourceView = twoResourceView()
  hud.setState({ View: newResourceView })
  sidebarLinks = hud.find(".Sidebar Link")
  expect(sidebarLinks).toHaveLength(3)
})

function oneResourceView() {
  const ts = Date.now().toLocaleString()
  const resource = {
    Name: "vigoda",
    DirectoriesWatched: ["foo", "bar"],
    LastDeployTime: ts,
    BuildHistory: [
      {
        Edits: ["main.go", "cli.go"],
        Error: "the build failed!",
        FinishTime: ts,
        StartTime: ts,
      },
    ],
    PendingBuildEdits: ["main.go", "cli.go", "vigoda.go"],
    PendingBuildSince: ts,
    CurrentBuild: {
      Edits: ["main.go"],
      StartTime: ts,
    },
    PodName: "vigoda-pod",
    PodCreationTime: ts,
    PodStatus: "Running",
    PodRestarts: 1,
    Endpoints: ["1.2.3.4:8080"],
    PodLog: "1\n2\n3\n4\nabe vigoda is now dead\n5\n6\n7\n8\n",
  }
  return { Resources: [resource] }
}

function twoResourceView() {
  const ts = Date.now().toLocaleString()
  const vigoda = {
    Name: "vigoda",
    DirectoriesWatched: ["foo", "bar"],
    LastDeployTime: ts,
    BuildHistory: [
      {
        Edits: ["main.go", "cli.go"],
        Error: "the build failed!",
        FinishTime: ts,
        StartTime: ts,
      },
    ],
    PendingBuildEdits: ["main.go", "cli.go", "vigoda.go"],
    PendingBuildSince: ts,
    CurrentBuild: {
      Edits: ["main.go"],
      StartTime: ts,
    },
    PodName: "vigoda-pod",
    PodCreationTime: ts,
    PodStatus: "Running",
    PodRestarts: 1,
    Endpoints: ["1.2.3.4:8080"],
    PodLog: "1\n2\n3\n4\nabe vigoda is now dead\n5\n6\n7\n8\n",
  }

  const snack = {
    Name: "snack",
    DirectoriesWatched: ["foo", "bar"],
    LastDeployTime: ts,
    BuildHistory: [
      {
        Edits: ["main.go", "cli.go"],
        Error: "the build failed!",
        FinishTime: ts,
        StartTime: ts,
      },
    ],
    PendingBuildEdits: ["main.go", "cli.go", "snack.go"],
    PendingBuildSince: ts,
    CurrentBuild: {
      Edits: ["main.go"],
      StartTime: ts,
    },
    PodName: "snack-pod",
    PodCreationTime: ts,
    PodStatus: "Running",
    PodRestarts: 1,
    Endpoints: ["1.2.3.4:8080"],
    PodLog: "1\n2\n3\n4\nsnacks are great\n5\n6\n7\n8\n",
  }
  return { Resources: [vigoda, snack] }
}
