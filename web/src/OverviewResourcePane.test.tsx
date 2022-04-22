import { render, RenderOptions, screen } from "@testing-library/react"
import { SnackbarProvider } from "notistack"
import React from "react"
import { MemoryRouter } from "react-router-dom"
import LogStore, { LogStoreProvider } from "./LogStore"
import OverviewResourcePane from "./OverviewResourcePane"
import { ResourceNavProvider } from "./ResourceNav"
import { nResourceView, oneResourceView, TestDataView } from "./testdata"
import { appendLinesForManifestAndSpan, Line } from "./testlogs"
import { LogLevel, UIResource } from "./types"

function customRender(
  options: {
    logStore?: LogStore
    selectedResource?: string
    view: TestDataView
  },
  renderOptions?: RenderOptions
) {
  const { logStore, view, selectedResource } = options
  const routerEntry = selectedResource
    ? `/r/${selectedResource}/overview`
    : "/overview"
  const validateResource = (name: string) =>
    view.uiResources?.some((res) => res.metadata?.name == name)

  return render(<OverviewResourcePane view={view} isSocketConnected={true} />, {
    wrapper: ({ children }) => (
      <MemoryRouter initialEntries={[routerEntry]}>
        <LogStoreProvider value={logStore ?? new LogStore()}>
          <SnackbarProvider>
            <ResourceNavProvider validateResource={validateResource}>
              {children}
            </ResourceNavProvider>
          </SnackbarProvider>
        </LogStoreProvider>
      </MemoryRouter>
    ),
    ...renderOptions,
  })
}

describe("OverviewResourcePane", () => {
  it("renders 'not found' message when trying to view a resource that doesn't exist", () => {
    customRender({ selectedResource: "does-not-exist", view: nResourceView(2) })

    expect(screen.getByText("No resource 'does-not-exist'")).toBeInTheDocument()
  })
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

    customRender({ view, logStore, selectedResource: r.metadata?.name })

    const errorFilterButton = screen.getByRole("button", { name: /errors/i })
    const warningFilterButton = screen.getByRole("button", {
      name: /warnings/i,
    })

    expect(errorFilterButton).toHaveTextContent(`Errors (${expectedErrs})`)
    expect(warningFilterButton).toHaveTextContent(`Warnings (${expectedWarns})`)
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
})
