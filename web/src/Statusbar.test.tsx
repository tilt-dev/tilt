import React from "react"
import ReactDOM from "react-dom"
import renderer from "react-test-renderer"
import Statusbar, { StatusItem } from "./Statusbar"
import { mount } from "enzyme"
import { oneResourceView, twoResourceView } from "./testdata.test"

describe("StatusBar", () => {
  it("renders without crashing", () => {
    const tree = renderer
      .create(<Statusbar items={[]} toggleSidebar={null} />)
      .toJSON()

    expect(tree).toMatchSnapshot()
  })

  it("renders two items both errors", () => {
    let items = twoResourceView().Resources.map(res => new StatusItem(res))
    let statusbar = mount(<Statusbar items={items} toggleSidebar={null} />)
    expect(statusbar.find(".Statusbar-panel--error").html()).toContain(
      "2 Errors"
    )
  })

  it("renders two items both errors snapshot", () => {
    let items = twoResourceView().Resources.map(res => new StatusItem(res))
    const tree = renderer
      .create(<Statusbar items={items} toggleSidebar={null} />)
      .toJSON()

    expect(tree).toMatchSnapshot()
  })

  it("renders two items all ok snapshot", () => {
    let view = twoResourceView()
    view.Resources.forEach(res => {
      res.BuildHistory[0].Error = ""
    })

    let items = view.Resources.map(res => new StatusItem(res))
    const tree = renderer
      .create(<Statusbar items={items} toggleSidebar={null} />)
      .toJSON()

    expect(tree).toMatchSnapshot()
  })

  it("renders two items all ok", () => {
    let view = twoResourceView()
    view.Resources.forEach(res => {
      res.BuildHistory[0].Error = ""
    })
    let items = view.Resources.map(res => new StatusItem(res))
    let statusbar = mount(<Statusbar items={items} toggleSidebar={null} />)
    expect(statusbar.find(".Statusbar-panel--error").html()).toContain(
      "0 Errors"
    )
  })
})

describe("StatusItem", () => {
  it("can be constructed with no build history", () => {
    let si = new StatusItem({})
    expect(si.hasError).toBe(false)
  })
})
