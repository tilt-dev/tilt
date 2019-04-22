import React from "react"
import ReactDOM from "react-dom"
import HUD from "./HUD"
import { mount } from "enzyme"
import { RouteComponentProps } from "react-router-dom"
import { oneResourceView, twoResourceView } from "./testdata.test"

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
  let sidebarLinks = hud.find(".Sidebar-resources Link")
  expect(sidebarLinks).toHaveLength(2)

  let newResourceView = twoResourceView()
  hud.setState({ View: newResourceView })
  sidebarLinks = hud.find(".Sidebar-resources Link")
  expect(sidebarLinks).toHaveLength(3)
})

it("renders tab nav", () => {
  const hud = mount(emptyHUD())

  let resourceView = oneResourceView()
  hud.setState({ View: resourceView })
  let tabNavLinks = hud.find(".TabNav Link")
  expect(tabNavLinks).toHaveLength(3)
})
