import { mount } from "enzyme"
import { MemoryRouter } from "react-router-dom"
import { FilterLevel } from "./logfilters"
import LogStore, { LogStoreProvider } from "./LogStore"
import { ButtonLeftPill, FilterRadioButton } from "./OverviewActionBar"
import OverviewResourcePane from "./OverviewResourcePane"
import { NotFound } from "./OverviewResourcePane.stories"
import OverviewResourceSidebar from "./OverviewResourceSidebar"
import { ResourceNavProvider } from "./ResourceNav"
import { oneResourceView } from "./testdata"
import { appendLinesForManifestAndSpan } from "./testlogs"

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
    prepare: (logStore: LogStore, r: Proto.webviewResource) => any
  ) => {
    const logStore = new LogStore()
    const view = oneResourceView()

    const r = view.resources[0]

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
      const latestBuild = r.buildHistory![0]
      latestBuild.spanId = "build:1"
      latestBuild.error = undefined
      latestBuild.warnings = []

      appendLinesForManifestAndSpan(logStore, r.name!, "build:1", [
        "the build failed!",
      ])
    })
  })

  it("creates alerts for build failures with existing spans", () => {
    doTest(1, 2, (logStore, r) => {
      const latestBuild = r.buildHistory![0]
      latestBuild.spanId = "build:1"
      latestBuild.error = "the build failed!"
      latestBuild.warnings = ["warning 1!", "warning 2!"]

      appendLinesForManifestAndSpan(logStore, r.name!, "build:1", [
        "the build failed!",
      ])
    })
  })

  it("ignores alerts for removed spans", () => {
    doTest(0, 0, (logStore, r) => {
      const latestBuild = r.buildHistory![0]
      latestBuild.spanId = "build:1"
      latestBuild.error = "the build failed!"
      latestBuild.warnings = ["warning!"]

      appendLinesForManifestAndSpan(logStore, r.name!, "build:2", [
        "the build failed!",
      ])
    })
  })
})
