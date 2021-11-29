import { mount } from "enzyme"
import { SnackbarProvider } from "notistack"
import React from "react"
import { MemoryRouter } from "react-router-dom"
import { FilterLevel } from "./logfilters"
import LogStore, { LogStoreProvider } from "./LogStore"
import OverviewActionBar, {
  ButtonLeftPill,
  FilterRadioButton,
} from "./OverviewActionBar"
import OverviewResourcePane from "./OverviewResourcePane"
import { NotFound } from "./OverviewResourcePane.stories"
import OverviewResourceSidebar from "./OverviewResourceSidebar"
import { ResourceNavContextProvider, ResourceNavProvider } from "./ResourceNav"
import {
  disableButton,
  oneButton,
  oneResource,
  oneResourceView,
} from "./testdata"
import { appendLinesForManifestAndSpan, Line } from "./testlogs"
import { LogLevel } from "./types"

type UIResource = Proto.v1alpha1UIResource

it("renders correctly when no resource found", () => {
  let root = mount(
    <MemoryRouter initialEntries={["/"]}>{NotFound()}</MemoryRouter>
  )
  let el = root.getDOMNode()
  expect(el.innerHTML).toEqual(
    expect.stringContaining("No resource 'does-not-exist'")
  )

  expect(root.find(OverviewResourceSidebar)).toHaveLength(1)
})

describe("alert filtering", () => {
  const doTest = (
    expectedErrs: number,
    expectedWarns: number,
    prepare: (logStore: LogStore, r: UIResource) => any
  ) => {
    const logStore = new LogStore()
    const view = oneResourceView()

    const r = view.uiResources[0]

    prepare(logStore, r)

    let root = mount(
      <MemoryRouter initialEntries={["/"]}>
        <LogStoreProvider value={logStore}>
          <ResourceNavProvider validateResource={() => true}>
            <OverviewResourcePane view={view} />
          </ResourceNavProvider>
        </LogStoreProvider>
      </MemoryRouter>
    )
    let errorFilter = root
      .find(FilterRadioButton)
      .filter({ level: FilterLevel.error })
      .find(ButtonLeftPill)
    let warnFilter = root
      .find(FilterRadioButton)
      .filter({ level: FilterLevel.warn })
      .find(ButtonLeftPill)

    expect(errorFilter.text()).toEqual(`Errors (${expectedErrs})`)
    expect(warnFilter.text()).toEqual(`Warnings (${expectedWarns})`)
  }

  it("creates no alerts if no build failures", () => {
    doTest(0, 0, (logStore, r) => {
      const latestBuild = r.status!.buildHistory![0] || {}
      latestBuild.spanID = "build:1"
      latestBuild.error = undefined
      latestBuild.warnings = []

      appendLinesForManifestAndSpan(logStore, r.metadata!.name!, "build:1", [
        "the build is ok!\n",
      ])
    })
  })

  it("creates alerts for build failures with existing spans", () => {
    doTest(1, 2, (logStore, r) => {
      const latestBuild = r.status!.buildHistory![0]
      latestBuild.spanID = "build:1"
      latestBuild.error = "the build failed!"
      latestBuild.warnings = ["warning 1!", "warning 2!"]

      appendLinesForManifestAndSpan(logStore, r.metadata!.name!, "build:1", [
        { level: LogLevel.WARN, anchor: true, text: "warning 1!\n" } as Line,
        { level: LogLevel.WARN, anchor: true, text: "warning 2!\n" } as Line,
        {
          level: LogLevel.ERROR,
          anchor: true,
          text: "the build failed!\n",
        } as Line,
      ])
    })
  })

  it("ignores alerts for removed spans", () => {
    doTest(0, 0, (logStore, r) => {
      const latestBuild = r.status!.buildHistory![0] || {}
      latestBuild.spanID = "build:1"
      latestBuild.error = "the build failed!"
      latestBuild.warnings = ["warning!"]

      appendLinesForManifestAndSpan(logStore, r.metadata!.name!, "build:2", [
        { level: LogLevel.WARN, anchor: true, text: "warning!\n" } as Line,
        {
          level: LogLevel.ERROR,
          anchor: true,
          text: "the build failed!\n",
        } as Line,
      ])
    })
  })

  it("categorizes buttons", () => {
    const view = {
      uiResources: [oneResource({ isBuilding: true })],
      uiButtons: [oneButton(0, "vigoda"), disableButton("vigoda", true)],
    }
    const root = mount(
      <MemoryRouter initialEntries={["/"]}>
        <LogStoreProvider value={new LogStore()}>
          <SnackbarProvider>
            <ResourceNavContextProvider
              value={{
                selectedResource: "vigoda",
                invalidResource: "",
                openResource: () => {},
              }}
            >
              <OverviewResourcePane view={view} />
            </ResourceNavContextProvider>
          </SnackbarProvider>
        </LogStoreProvider>
      </MemoryRouter>
    )

    const b = root.find(OverviewActionBar)
    const buttons = root.find(OverviewActionBar).prop("buttons")
    expect(buttons?.default).toEqual([view.uiButtons[0]])
    expect(buttons?.toggleDisable).toEqual(view.uiButtons[1])
  })
})
