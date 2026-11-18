import {
  fireEvent,
  render,
  RenderOptions,
  screen,
  waitFor,
  within,
} from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import fetchMock from "fetch-mock"
import { SnackbarProvider } from "notistack"
import React from "react"
import { MemoryRouter, useLocation } from "react-router-dom"
import Features, { FeaturesTestProvider, Flag } from "./feature"
import LogStore, { LogStoreProvider } from "./LogStore"
import OverviewTablePane, { snapWhileDragging } from "./OverviewTablePane"
import { ResourceGroupsContextProvider } from "./ResourceGroupsContext"
import {
  DEFAULT_OPTIONS,
  ResourceListOptions,
  ResourceListOptionsProvider,
} from "./ResourceListOptionsContext"
import { ResourceNavProvider } from "./ResourceNav"
import { ResourceSelectionProvider } from "./ResourceSelectionContext"
import { StarredResourceMemoryProvider } from "./StarredResourcesContext"
import {
  nResourceView,
  oneResourceFailedToBuild,
  oneResourceView,
  TestDataView,
} from "./testdata"
import { appendLinesForManifestAndSpan, Line } from "./testlogs"
import { CONDENSED_SNAP_WIDTHS } from "./OverviewTableColumns"
import { LogLevel, UIResource, UpdateStatus } from "./types"

function CurrentPath() {
  const location = useLocation()
  return (
    <div data-testid="current-path">{location.pathname + location.search}</div>
  )
}

function customRender(
  options: {
    view: TestDataView
    path?: string
    logStore?: LogStore
    starred?: string[]
    resourceListOptions?: ResourceListOptions
  },
  renderOptions?: RenderOptions
) {
  const { view } = options
  const validateResource = (name: string) =>
    !!view.uiResources?.some((res) => res.metadata?.name == name)
  const features = new Features({ [Flag.Labels]: true })

  return render(<OverviewTablePane view={view} isSocketConnected={true} />, {
    wrapper: ({ children }) => (
      <MemoryRouter
        initialEntries={[options.path ?? "/overview"]}
        future={{ v7_startTransition: true, v7_relativeSplatPath: true }}
      >
        <FeaturesTestProvider value={features}>
          <LogStoreProvider value={options.logStore ?? new LogStore()}>
            <SnackbarProvider>
              <StarredResourceMemoryProvider
                initialValueForTesting={options.starred}
              >
                <ResourceNavProvider validateResource={validateResource}>
                  <ResourceGroupsContextProvider>
                    <ResourceListOptionsProvider
                      initialValuesForTesting={options.resourceListOptions}
                    >
                      <ResourceSelectionProvider>
                        {children}
                        <CurrentPath />
                      </ResourceSelectionProvider>
                    </ResourceListOptionsProvider>
                  </ResourceGroupsContextProvider>
                </ResourceNavProvider>
              </StarredResourceMemoryProvider>
            </SnackbarProvider>
          </LogStoreProvider>
        </FeaturesTestProvider>
      </MemoryRouter>
    ),
    ...renderOptions,
  })
}

const currentPath = () => screen.getByTestId("current-path").textContent
const detailsPanel = (name: string) =>
  screen.queryByRole("region", { name: `Details for ${name}` })

afterEach(() => {
  sessionStorage.clear()
  localStorage.clear()
})

describe("details panel", () => {
  it("shows only the table when no resource is open", () => {
    customRender({ view: nResourceView(3) })

    expect(screen.getByRole("table")).toBeInTheDocument()
    expect(screen.queryByLabelText(/^Details for/)).not.toBeInTheDocument()
  })

  it("opens a resource's details when its row is clicked", async () => {
    customRender({ view: nResourceView(3) })

    userEvent.click(screen.getByText("_1"))

    expect(currentPath()).toBe("/r/_1/overview")
    const panel = detailsPanel("_1")
    expect(panel).toBeInTheDocument()
    expect(
      within(panel!).getByRole("heading", { name: "_1" })
    ).toBeInTheDocument()
  })

  it("condenses the table while the details are open", () => {
    customRender({ view: nResourceView(3), path: "/r/_1/overview" })

    // The bulk selection checkboxes and wide columns go away
    expect(screen.queryByTitle("Sort by Pod ID")).not.toBeInTheDocument()
    expect(screen.queryByLabelText("Select all resources")).toBeNull()
    // The open row is highlighted
    expect(screen.getByTitle("_1").closest("tr")).toHaveClass("isOpen")
  })

  it("closes on Esc, bringing back the full table", async () => {
    customRender({ view: nResourceView(3), path: "/r/_1/overview" })

    userEvent.keyboard("{Escape}")

    expect(currentPath()).toBe("/overview")
    await waitFor(() => expect(detailsPanel("_1")).not.toBeInTheDocument())
    expect(screen.getByTitle("Sort by Pod ID")).toBeInTheDocument()
  })

  it("closes with the close button", () => {
    customRender({ view: nResourceView(3), path: "/r/_1/overview" })

    userEvent.click(screen.getByLabelText("Close details (Esc)"))

    expect(currentPath()).toBe("/overview")
  })

  it("only leaves a text field on the first Esc", () => {
    customRender({ view: nResourceView(3), path: "/r/_1/overview" })

    const logFilter = screen.getByPlaceholderText("Filter by text or /regexp/")
    userEvent.click(logFilter)
    userEvent.keyboard("{Escape}")

    expect(logFilter).not.toHaveFocus()
    expect(currentPath()).toBe("/r/_1/overview")

    userEvent.keyboard("{Escape}")
    expect(currentPath()).toBe("/overview")
  })

  it("says when a resource doesn't exist", () => {
    customRender({ view: nResourceView(2), path: "/r/does-not-exist/overview" })

    expect(screen.getByText("No resource 'does-not-exist'")).toBeInTheDocument()
  })

  it("collapses and expands the resource list", async () => {
    customRender({ view: nResourceView(3), path: "/r/_1/overview" })

    userEvent.click(screen.getByLabelText("Collapse resource list"))
    // Once it has finished shrinking, only the expand button is left
    await waitFor(() => expect(screen.getByTitle("_2")).not.toBeVisible())

    userEvent.click(screen.getByLabelText("Expand resource list"))
    expect(screen.getByTitle("_2")).toBeVisible()
    await waitFor(() =>
      expect(screen.getByLabelText("Collapse resource list")).toBeVisible()
    )
  })
})

describe("details header", () => {
  it("shows the resource's URL prominently, with a copy button", async () => {
    const writeText = jest.fn().mockResolvedValue(undefined)
    Object.assign(navigator, { clipboard: { writeText } })
    customRender({ view: nResourceView(3), path: "/r/_1/overview" })

    const urls = screen.getByLabelText("URLs")
    const link = within(urls).getByRole("link")
    expect(link).toHaveTextContent("1.2.3.4:8080")
    userEvent.click(within(urls).getByLabelText("Copy URL"))
    expect(writeText).toHaveBeenCalledWith(link.getAttribute("href"))
    expect(await within(urls).findByLabelText("Copied!")).toBeInTheDocument()
  })

  it("leaves type and mode to the table", () => {
    customRender({ view: nResourceView(3), path: "/r/_1/overview" })
    const header = screen.getByRole("banner", { name: "_1 details" })
    expect(within(header).queryByText("Type")).toBeNull()
    expect(within(header).queryByText("Mode")).toBeNull()
  })
})

describe("details header actions", () => {
  afterEach(() => fetchMock.reset())

  const header = () => screen.getByRole("banner", { name: "_1 details" })

  it("stars the resource from next to its name", () => {
    customRender({ view: nResourceView(3), path: "/r/_1/overview" })
    userEvent.click(within(header()).getByTitle("Star this Resource"))
    expect(within(header()).getByTitle("Remove Star")).toBeInTheDocument()
  })

  it("shows when the resource last updated", () => {
    customRender({ view: nResourceView(3), path: "/r/_1/overview" })
    expect(within(header()).getByLabelText("Last updated")).toHaveTextContent(
      "ago"
    )
  })

  it("mentions the re-run shortcut", () => {
    customRender({ view: nResourceView(3), path: "/r/_1/overview" })
    const rerun = within(header()).getByRole("button", {
      name: "Trigger update (r)",
    })
    expect(rerun).toHaveAttribute("aria-keyshortcuts", "r")
  })

  it("switches trigger mode from the more menu", () => {
    fetchMock.mock("/api/override/trigger_mode", JSON.stringify({}))
    customRender({ view: nResourceView(3), path: "/r/_1/overview" })
    userEvent.click(within(header()).getByLabelText("More actions"))
    userEvent.click(screen.getByText("Switch to manual updates"))
    expect(fetchMock.calls().length).toEqual(1)
    expect(JSON.parse(fetchMock.calls()[0][1]?.body as string)).toEqual({
      manifest_names: ["_1"],
      trigger_mode: 2,
    })
  })

  it("asks before disabling from the more menu", async () => {
    fetchMock.mock("*", JSON.stringify({}))
    customRender({ view: nResourceView(3), path: "/r/_1/overview" })
    expect(within(header()).queryByText("Disable Resource")).toBeNull()
    userEvent.click(within(header()).getByLabelText("More actions"))
    userEvent.click(screen.getByText("Disable resource"))
    expect(fetchMock.calls().length).toEqual(0)
    userEvent.click(screen.getByText("Click again to disable"))
    await waitFor(() => expect(fetchMock.calls().length).toEqual(1))
    expect(fetchMock.calls()[0][0]).toContain("toggle-_1-disable")
  })

  it("keeps the pod ID under show more", () => {
    customRender({ view: nResourceView(3), path: "/r/_1/overview" })
    expect(within(header()).queryByText("Pod ID")).toBeNull()
    userEvent.click(within(header()).getByText("Show more"))
    expect(within(header()).getByText("Pod ID")).toBeInTheDocument()
    userEvent.click(within(header()).getByText("Show less"))
    expect(within(header()).queryByText("Pod ID")).toBeNull()
  })
})

describe("collapsed resource list", () => {
  it("shows a red dot for each resource with an error", async () => {
    const view = nResourceView(3)
    const broken = oneResourceFailedToBuild()[0]
    broken.status!.updateStatus = UpdateStatus.Error
    view.uiResources = [...view.uiResources!, broken]
    customRender({ view, path: "/r/_1/overview" })

    userEvent.click(screen.getByLabelText("Collapse resource list"))
    const rail = await screen.findByLabelText("Resources with errors")
    const dots = within(rail).getAllByRole("link")
    expect(dots.map((d) => d.getAttribute("aria-label"))).toEqual([
      "snack: error",
    ])
    userEvent.click(dots[0])
    expect(screen.getByTestId("current-path")).toHaveTextContent(
      "/r/snack/overview"
    )
  })

  it("stays empty when nothing is broken", async () => {
    customRender({ view: nResourceView(3), path: "/r/_1/overview" })
    userEvent.click(screen.getByLabelText("Collapse resource list"))
    await screen.findByLabelText("Expand resource list")
    expect(screen.queryByLabelText("Resources with errors")).toBeNull()
  })
})

describe("list width on a laptop", () => {
  const innerWidth = window.innerWidth
  afterEach(() => {
    window.innerWidth = innerWidth
  })

  it("starts narrow on a 14-inch screen, with Updated one click away", () => {
    window.innerWidth = 1512
    customRender({ view: nResourceView(3), path: "/r/_1/overview" })
    const showUpdated = screen.getByLabelText("Show hidden columns: Updated")
    userEvent.click(showUpdated)
    expect(screen.queryByLabelText(/Show hidden columns/)).toBeNull()
  })

  it("starts with every column on a big screen", () => {
    window.innerWidth = 1800
    customRender({ view: nResourceView(3), path: "/r/_1/overview" })
    expect(screen.queryByLabelText(/Show hidden columns/)).toBeNull()
  })
})

describe("starred resources bar", () => {
  it("shows above the table, but not with the details open", () => {
    customRender({ view: nResourceView(3), starred: ["_1"] })
    expect(screen.getByLabelText("Starred resources")).toBeInTheDocument()
    customRender({
      view: nResourceView(3),
      starred: ["_1"],
      path: "/r/_2/overview",
    })
    expect(screen.getAllByLabelText("Starred resources")).toHaveLength(1)
  })
})

describe("dragging the divider", () => {
  const [icons, nameAndStatus, withTrigger, full] = CONDENSED_SNAP_WIDTHS

  it("moves in steps, one per layout", () => {
    expect(snapWhileDragging(full - 30, full)).toBe(full)
    expect(snapWhileDragging(withTrigger + 5, full)).toBe(withTrigger)
    expect(snapWhileDragging(icons - 40, nameAndStatus)).toBe(icons)
  })

  it("doesn't flicker when the pointer wobbles around halfway", () => {
    const halfway = (nameAndStatus + withTrigger) / 2
    expect(snapWhileDragging(halfway + 5, nameAndStatus)).toBe(nameAndStatus)
    expect(snapWhileDragging(halfway - 5, withTrigger)).toBe(withTrigger)
  })

  it("follows the pointer past the full width", () => {
    expect(snapWhileDragging(full + 40, full)).toBe(full + 40)
  })
})

describe("narrow resource list", () => {
  const innerWidth = window.innerWidth
  beforeEach(() => {
    window.innerWidth = 1800
  })
  afterEach(() => {
    window.innerWidth = innerWidth
  })

  it("steps between whole-column widths and offers to bring hidden columns back", async () => {
    customRender({ view: nResourceView(3), path: "/r/_1/overview" })
    const divider = screen.getByRole("separator", {
      name: "Resize resource list",
    })
    expect(screen.queryByLabelText(/Show hidden columns/)).toBeNull()

    // Past the full width, arrows move in small steps; below it, they jump
    // to the next whole-column width, which hides "Updated".
    for (let i = 0; i < 5; i++) {
      fireEvent.keyDown(divider, { key: "ArrowLeft" })
    }
    const showButton = await screen.findByLabelText(
      "Show hidden columns: Updated"
    )
    expect(screen.queryAllByTitle("Sort by Updated")).toHaveLength(0)
    expect(screen.getAllByTitle("Sort by Status").length).toBeGreaterThan(0)

    userEvent.click(showButton)
    await waitFor(() =>
      expect(screen.getAllByTitle("Sort by Updated").length).toBeGreaterThan(0)
    )
    expect(screen.queryByLabelText(/Show hidden columns/)).toBeNull()
  })
})

describe("returning to the details", () => {
  const detailButton = () => screen.getByRole("link", { name: "Detail view" })

  it("opens all resources if nothing was opened yet", () => {
    customRender({ view: nResourceView(3) })
    expect(detailButton()).toHaveAttribute("href", "/r/(all)/overview")
  })

  it("reopens the last resource, and marks its row in the table", async () => {
    customRender({ view: nResourceView(3), path: "/r/_2/overview" })

    userEvent.click(screen.getByRole("link", { name: "Table view" }))
    await waitFor(() => expect(detailsPanel("_2")).toBeNull())
    expect(screen.getByTitle("_2").closest("tr")).toHaveClass("wasOpen")
    expect(detailButton()).toHaveAttribute("href", "/r/_2/overview")

    userEvent.click(detailButton())
    await waitFor(() => expect(detailsPanel("_2")).toBeInTheDocument())
  })

  it("falls back to all resources if the last one is gone", () => {
    customRender({ view: nResourceView(3), path: "/r/does-not-exist/overview" })
    expect(detailButton()).toHaveAttribute("href", "/r/(all)/overview")
  })
})

describe("combined logs", () => {
  it("lists All Resources above the table", () => {
    customRender({ view: nResourceView(3) })

    userEvent.click(screen.getByLabelText("View all resource logs"))

    expect(currentPath()).toBe("/r/(all)/overview")
    expect(detailsPanel("(all)")).toBeInTheDocument()
    expect(screen.getByLabelText("View all resource logs")).toHaveClass(
      "isOpen"
    )
  })

  it("lists Starred Resources only when something is starred", () => {
    customRender({ view: nResourceView(3) })
    expect(screen.queryByLabelText("View starred resource logs")).toBeNull()
  })

  it("opens the starred resources' logs", () => {
    customRender({ view: nResourceView(3), starred: ["_2"] })

    userEvent.click(screen.getByLabelText("View starred resource logs"))

    expect(currentPath()).toBe("/r/(starred)/overview")
    const panel = detailsPanel("(starred)")
    expect(
      within(panel!).getByRole("heading", { name: "Starred Resources" })
    ).toBeInTheDocument()
  })
})

describe("keyboard", () => {
  beforeEach(() => {
    fetchMock.mock("/api/trigger", JSON.stringify({}))
  })

  afterEach(() => {
    fetchMock.reset()
  })

  it("moves through the combined logs and resources with j and k", () => {
    customRender({
      view: nResourceView(3),
      path: "/r/(all)/overview",
      starred: ["_2"],
    })

    userEvent.keyboard("j")
    expect(currentPath()).toBe("/r/(starred)/overview")
    userEvent.keyboard("j")
    expect(currentPath()).toBe("/r/(Tiltfile)/overview")
    userEvent.keyboard("j")
    expect(currentPath()).toBe("/r/_1/overview")
    userEvent.keyboard("k")
    expect(currentPath()).toBe("/r/(Tiltfile)/overview")
  })

  it("moves down with the down arrow", () => {
    customRender({ view: nResourceView(3), path: "/r/_1/overview" })

    userEvent.keyboard("{ArrowDown}")

    expect(currentPath()).toBe("/r/_2/overview")
  })

  it("re-runs the open resource on 'r'", () => {
    customRender({ view: nResourceView(3), path: "/r/_1/overview" })

    userEvent.keyboard("r")

    expect(fetchMock.calls().length).toEqual(1)
    expect(fetchMock.calls()[0][1]?.body).toEqual(
      JSON.stringify({ manifest_names: ["_1"], build_reason: 16 /* manual */ })
    )
  })

  it("doesn't re-run anything from the combined logs", () => {
    customRender({ view: nResourceView(3), path: "/r/(all)/overview" })

    userEvent.keyboard("r")

    expect(fetchMock.calls().length).toEqual(0)
  })
})

describe("alerts on top", () => {
  it("moves resources with alerts to the top of the list", () => {
    const logStore = new LogStore()
    const view = nResourceView(4)
    const failing = view.uiResources[3]
    failing.status!.buildHistory![0].spanID = "build:3"
    appendLinesForManifestAndSpan(logStore, "_3", "build:3", [
      { level: LogLevel.ERROR, anchor: true, text: "oh no\n" } as Line,
    ])

    customRender({
      view,
      logStore,
      resourceListOptions: { ...DEFAULT_OPTIONS, alertsOnTop: true },
    })

    const rowNames = screen
      .getAllByRole("row")
      .map((row) => row.textContent?.match(/_\d/)?.[0])
      .filter((name) => name)
    expect(rowNames).toEqual(["_3", "_1", "_2"])
  })
})

describe("alert counts", () => {
  const doTest = (
    expectedErrs: number,
    expectedWarns: number,
    prepare: (logStore: LogStore, r: UIResource) => any
  ) => {
    const logStore = new LogStore()
    const view = oneResourceView()
    const r = view.uiResources[0]
    prepare(logStore, r)

    customRender({ view, logStore, path: `/r/${r.metadata?.name}/overview` })

    expect(screen.getByRole("button", { name: /errors/i })).toHaveTextContent(
      `Errors (${expectedErrs})`
    )
    expect(screen.getByRole("button", { name: /warnings/i })).toHaveTextContent(
      `Warnings (${expectedWarns})`
    )
  }

  it("counts no alerts if no build failures", () => {
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

  it("counts alerts for build failures with existing spans", () => {
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
})
