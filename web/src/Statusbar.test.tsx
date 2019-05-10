import React from "react"
import renderer from "react-test-renderer"
import Statusbar, { StatusItem } from "./Statusbar"
import { mount } from "enzyme"
import { twoResourceView } from "./testdata.test"
import { MemoryRouter } from "react-router"
import { TiltBuild } from "./types"
import moment, {Duration} from "moment"
import { SinonFakeTimers, useFakeTimers } from "sinon"

describe("StatusBar", () => {
  var clock: SinonFakeTimers
  beforeEach(() => {
    clock = useFakeTimers()
  })
  afterEach(() => {
    clock.restore()
  })

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
            errorsUrl="/errors"
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
          errorsUrl="/errors"
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
            errorsUrl="/errors"
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
            errorsUrl="/errors"
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
          errorsUrl="/errors"
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

  test.each([2, 6, 10])(
    "notifies of update with appropriate color when %d days old",
    daysOld => {
      let view = twoResourceView()
      view.Resources.forEach((res: any) => {
        res.BuildHistory[0].Error = ""
      })
      let items = view.Resources.map((res: any) => new StatusItem(res))

      let latestVersion: TiltBuild = {
        Version: "v0.10.0",
        Date: moment(Date.now())
          .subtract(daysOld, "days")
          .format("YYYY-MM-DD"),
        Dev: false,
      }
      const tree = renderer
        .create(
          <MemoryRouter>
            <Statusbar
              items={[]}
              errorsUrl="/errors"
              runningVersion={runningVersion}
              latestVersion={latestVersion}
            />
          </MemoryRouter>
        )
        .toJSON()
      expect(tree).toMatchSnapshot()
    }
  )

  it("updates as time passes", () => {
    let view = twoResourceView()
    view.Resources.forEach((res: any) => {
      res.BuildHistory[0].Error = ""
    })
    let items = view.Resources.map((res: any) => new StatusItem(res))
    let latestVersion: TiltBuild = {
      Version: "v0.10.0",
      Date: moment(Date.now())
        .subtract(1, "days")
        .format("YYYY-MM-DD"),
      Dev: false,
    }

    let statusbar = mount(
      <MemoryRouter>
        <Statusbar
          items={items}
          errorsUrl="/errors"
          runningVersion={runningVersion}
          latestVersion={latestVersion}
        />
      </MemoryRouter>
    )
    expect(
      statusbar.find(".Statusbar-updatePanel-outofdate-short").length
    ).toEqual(1)

    clock.tick(moment.duration(4, 'days').asMilliseconds())

    statusbar.update()

    expect(
      statusbar.find(".Statusbar-updatePanel-outofdate-short").length
    ).toEqual(0)
    expect(
      statusbar.find(".Statusbar-updatePanel-outofdate-medium").length
    ).toEqual(1)

    statusbar.unmount()
  })
})

describe("StatusItem", () => {
  it("can be constructed with no build history", () => {
    let si = new StatusItem({})
    expect(si.hasError).toBe(false)
  })
})
