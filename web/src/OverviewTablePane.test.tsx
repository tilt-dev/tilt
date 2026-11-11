import {
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
import OverviewTablePane from "./OverviewTablePane"
import { ResourceGroupsContextProvider } from "./ResourceGroupsContext"
import {
  DEFAULT_OPTIONS,
  ResourceListOptions,
  ResourceListOptionsProvider,
} from "./ResourceListOptionsContext"
import { ResourceNavProvider } from "./ResourceNav"
import { ResourceSelectionProvider } from "./ResourceSelectionContext"
import { StarredResourceMemoryProvider } from "./StarredResourcesContext"
import { nResourceView, oneResourceView, TestDataView } from "./testdata"
import { appendLinesForManifestAndSpan, Line } from "./testlogs"
import { LogLevel, UIResource } from "./types"

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
