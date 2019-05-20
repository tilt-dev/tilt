import React from "react"
import renderer from "react-test-renderer"
import Statusbar, { StatusItem } from "./Statusbar"
import { mount } from "enzyme"
import { twoResourceView } from "./testdata.test"
import { MemoryRouter } from "react-router"
import { TiltBuild } from "./types"

describe("StatusBar", () => {
  let runningVersion: TiltBuild = {
    Version: "v0.8.1",
    Date: "1970-01-01",
    Dev: false,
  }
  it("renders without crashing", () => {
    const tree = renderer
      .create(
        <MemoryRouter>
          <Statusbar
            items={[]}
            alertsUrl="/alerts"
            runningVersion={runningVersion}
            latestVersion={null}
          />
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
        <Statusbar
          items={items}
          alertsUrl="/alerts"
          runningVersion={runningVersion}
          latestVersion={null}
        />
      </MemoryRouter>
    )
    expect(
      statusbar.find(".Statusbar-errWarnPanel-count--error").html()
    ).toContain("2")

    statusbar.unmount()
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
          <Statusbar
            items={items}
            alertsUrl="/alerts"
            runningVersion={runningVersion}
            latestVersion={null}
          />
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
          <Statusbar
            items={items}
            alertsUrl="/alerts"
            runningVersion={runningVersion}
            latestVersion={null}
          />
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
        <Statusbar
          items={items}
          alertsUrl="/alerts"
          runningVersion={runningVersion}
          latestVersion={null}
        />
      </MemoryRouter>
    )
    expect(
      statusbar.find(".Statusbar-errWarnPanel-count--error").html()
    ).toContain("0")

    statusbar.unmount()
  })

  it("renders an upgrade badge when the version is out of date", () => {
    let view = twoResourceView()
    view.Resources.forEach((res: any) => {
      res.BuildHistory[0].Error = ""
    })
    let items = view.Resources.map((res: any) => new StatusItem(res))
    let latestVersion = runningVersion
    latestVersion.Version = "10.0.0"
    const tree = renderer
      .create(
        <MemoryRouter>
          <Statusbar
            items={items}
            alertsUrl="/alerts"
            runningVersion={runningVersion}
            latestVersion={latestVersion}
          />
        </MemoryRouter>
      )
      .toJSON()

    expect(tree).toMatchSnapshot()
  })

  it("does not render an upgrade badge when there is no latestVersion", () => {
    let view = twoResourceView()
    view.Resources.forEach((res: any) => {
      res.BuildHistory[0].Error = ""
    })
    let items = view.Resources.map((res: any) => new StatusItem(res))
    let latestVersion = { Version: "", Date: "", Dev: false }
    const tree = renderer
      .create(
        <MemoryRouter>
          <Statusbar
            items={items}
            alertsUrl="/alerts"
            runningVersion={runningVersion}
            latestVersion={latestVersion}
          />
        </MemoryRouter>
      )
      .toJSON()

    expect(tree).toMatchSnapshot()
  })

  it("does not render an upgrade badge when runningVersion is dev", () => {
    let view = twoResourceView()
    view.Resources.forEach((res: any) => {
      res.BuildHistory[0].Error = ""
    })
    let items = view.Resources.map((res: any) => new StatusItem(res))
    let latestVersion = runningVersion
    latestVersion.Version = "10.0.0"
    let devRunningVersion = runningVersion
    devRunningVersion.Dev = true
    const tree = renderer
      .create(
        <MemoryRouter>
          <Statusbar
            items={items}
            alertsUrl="/alerts"
            runningVersion={devRunningVersion}
            latestVersion={latestVersion}
          />
        </MemoryRouter>
      )
      .toJSON()

    expect(tree).toMatchSnapshot()
  })
})

describe("StatusItem", () => {
  it("can be constructed with no build history", () => {
    let si = new StatusItem({})
    expect(si.hasError).toBe(false)
  })
})
