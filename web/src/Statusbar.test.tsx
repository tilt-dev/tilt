import React from "react"
import ReactDOM from "react-dom"
import renderer from "react-test-renderer"
import Statusbar, { StatusItem } from "./Statusbar"
import { mount } from "enzyme"
import { oneResourceView, twoResourceView } from "./testdata.test"

describe("StatusBar", () => {
  it("renders without crashing", () => {
    const tree = renderer.create(<Statusbar items={[]} />).toJSON()

    expect(tree).toMatchSnapshot()
  })

  it("renders two items both errors", () => {
    let items = twoResourceView().Resources.map((res: any) => {
      res.CurrentBuild = {}
      res.PendingBuildSince = ""
      return new StatusItem(res)
    })
    let statusbar = mount(<Statusbar items={items} />)
    expect(
      statusbar.find(".Statusbar-panel .err-warn-item--error .count").html()
    ).toContain("2")
  })

  it("renders two items both errors snapshot", () => {
    let items = twoResourceView().Resources.map((res: any) => {
      res.CurrentBuild = {}
      res.PendingBuildSince = ""
      return new StatusItem(res)
    })
    const tree = renderer.create(<Statusbar items={items} />).toJSON()

    expect(tree).toMatchSnapshot()
  })

  it("renders two items all ok snapshot", () => {
    let view = twoResourceView()
    view.Resources.forEach((res: any) => {
      res.BuildHistory[0].Error = ""
    })

    let items = view.Resources.map((res: any) => new StatusItem(res))
    const tree = renderer.create(<Statusbar items={items} />).toJSON()

    expect(tree).toMatchSnapshot()
  })

  it("renders two items all ok", () => {
    let view = twoResourceView()
    view.Resources.forEach((res: any) => {
      res.BuildHistory[0].Error = ""
    })
    let items = view.Resources.map((res: any) => new StatusItem(res))
    let statusbar = mount(<Statusbar items={items} />)
    expect(
      statusbar.find(".Statusbar-panel .err-warn-item--error .count").html()
    ).toContain("0")
  })
})

describe("StatusItem", () => {
  it("can be constructed with no build history", () => {
    let si = new StatusItem({})
    expect(si.hasError).toBe(false)
  })
})
