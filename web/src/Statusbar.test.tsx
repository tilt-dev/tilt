import React from "react"
import ReactDOM from "react-dom"
import renderer from "react-test-renderer"
import Statusbar, { StatusItem } from "./Statusbar"
import { mount } from "enzyme"
import { oneResourceView, twoResourceView } from "./testdata.test"
import { MemoryRouter } from "react-router"

describe("StatusBar", () => {
  it("renders without crashing", () => {
    const tree = renderer
      .create(
        <MemoryRouter>
          <Statusbar items={[]} alertsUrl="/errors" />
        </MemoryRouter>
      )
      .toJSON()

    expect(tree).toMatchSnapshot()
  })

  it("renders two items both errors", () => {
    let items = twoResourceView().Resources.map((res: any) => {
      res.CurrentBuild = {}
      res.PendingBuildSince = ""
      return new StatusItem(res)
    })
    let statusbar = mount(
      <MemoryRouter>
        <Statusbar items={items} alertsUrl="/errors" />
      </MemoryRouter>
    )
    expect(
      statusbar.find(".Statusbar-errWarnPanel-count--error").html()
    ).toContain("2")
  })

  it("renders two items both errors snapshot", () => {
    let items = twoResourceView().Resources.map((res: any) => {
      res.CurrentBuild = {}
      res.PendingBuildSince = ""
      return new StatusItem(res)
    })
    const tree = renderer
      .create(
        <MemoryRouter>
          <Statusbar items={items} alertsUrl="/errors" />
        </MemoryRouter>
      )
      .toJSON()

    expect(tree).toMatchSnapshot()
  })

  it("renders two items all ok snapshot", () => {
    let view = twoResourceView()
    view.Resources.forEach((res: any) => {
      res.BuildHistory[0].Error = ""
    })

    let items = view.Resources.map((res: any) => new StatusItem(res))
    const tree = renderer
      .create(
        <MemoryRouter>
          <Statusbar items={items} alertsUrl="/errors" />
        </MemoryRouter>
      )
      .toJSON()

    expect(tree).toMatchSnapshot()
  })

  it("renders two items all ok", () => {
    let view = twoResourceView()
    view.Resources.forEach((res: any) => {
      res.BuildHistory[0].Error = ""
    })
    let items = view.Resources.map((res: any) => new StatusItem(res))
    let statusbar = mount(
      <MemoryRouter>
        <Statusbar items={items} alertsUrl="/errors" />
      </MemoryRouter>
    )
    expect(
      statusbar.find(".Statusbar-errWarnPanel-count--error").html()
    ).toContain("0")
  })
})

describe("StatusItem", () => {
  it("can be constructed with no build history", () => {
    let si = new StatusItem({})
    expect(si.hasError).toBe(false)
  })
})
