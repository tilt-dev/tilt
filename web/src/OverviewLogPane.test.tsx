import { render, RenderOptions, screen } from "@testing-library/react"
import { Component } from "react"
import { findRenderedComponentWithType } from "react-dom/test-utils"
import { MemoryRouter } from "react-router"
import {
  createFilterTermState,
  EMPTY_FILTER_TERM,
  FilterLevel,
  FilterSource,
} from "./logfilters"
import LogStore, { LogUpdateAction, LogStoreProvider } from "./LogStore"
import OverviewLogPane, {
  OverviewLogComponent,
  PROLOGUE_LENGTH,
  renderWindow,
} from "./OverviewLogPane"
import {
  BuildLogAndRunLog,
  ManyLines,
  StyledLines,
  ThreeLines,
  ThreeLinesAllLog,
  StarredResourcesLog,
  UrlWithAnsiInPort,
} from "./OverviewLogPane.stories"
import { newFakeRaf, RafProvider, SyncRafProvider, TestRafContext } from "./raf"
import { renderTestComponent } from "./test-helpers"
import { appendLines } from "./testlogs"

function customRender(component: JSX.Element, options?: RenderOptions) {
  return render(component, {
    wrapper: ({ children }) => (
      <MemoryRouter
        initialEntries={["/"]}
        future={{ v7_startTransition: true, v7_relativeSplatPath: true }}
      >
        <SyncRafProvider>{children}</SyncRafProvider>
      </MemoryRouter>
    ),
    ...options,
  })
}

describe("OverviewLogPane", () => {
  it("renders all log lines associated with a specific resource", () => {
    const { container } = customRender(<ThreeLines />)
    expect(container.querySelectorAll(".LogLine")).toHaveLength(3)
  })

  it("renders all log lines in the all log view", () => {
    const { container } = customRender(<ThreeLinesAllLog />)
    expect(container.querySelectorAll(".LogLine")).toHaveLength(3)
  })

  it("renders log lines of starred resources", () => {
    const { container } = customRender(<StarredResourcesLog />)
    expect(container.querySelectorAll(".LogLine")).toHaveLength(9)
  })

  it("escapes html and linkifies", () => {
    customRender(<StyledLines />)
    expect(screen.getAllByRole("link")).toHaveLength(3)
    expect(screen.queryByRole("button")).toBeNull()
  })

  it("linkifies URLs with ANSI in port (UrlWithAnsiInPort story)", () => {
    const { container } = customRender(<UrlWithAnsiInPort />)
    expect(
      container.querySelector('a[href="http://localhost:3000/"]')
    ).toBeTruthy()
    expect(
      container.querySelector('a[href="http://admin.localhost:3000/"]')
    ).toBeTruthy()
  })

  it("properly escapes ansi chars", () => {
    let defaultFilter = {
      source: FilterSource.all,
      level: FilterLevel.all,
      term: EMPTY_FILTER_TERM,
    }
    let logStore = new LogStore()
    appendLines(logStore, "fe", "[32m➜[39m  [1mLocal[22m:   [36mhttp://localhost:[1m5173[22m/[39m\n")
    const { container } = customRender(
      <LogStoreProvider value={logStore}>
        <OverviewLogPane manifestName="fe" filterSet={defaultFilter} />
      </LogStoreProvider>
    )
    expect(container.querySelectorAll(".LogLine")).toHaveLength(1)
    expect(container.querySelector(".LogLine")).toHaveTextContent(
      "➜ Local: http://localhost:5173/"
    )
  })

  it("displays all logs when there are no filters", () => {
    const { container } = customRender(<BuildLogAndRunLog />)
    expect(container.querySelectorAll(".LogLine")).toHaveLength(40)
  })

  describe("filters by source", () => {
    it("displays only runtime logs when runtime source is specified", () => {
      const { container } = customRender(
        <BuildLogAndRunLog
          level=""
          source={FilterSource.runtime}
          term={EMPTY_FILTER_TERM}
        />
      )
      expect(container.querySelectorAll(".LogLine")).toHaveLength(20)
      expect(screen.getAllByText(/Vigoda pod line/)).toHaveLength(18)
      expect(screen.queryByText(/Vigoda build line/)).toBeNull()
    })

    it("displays only build logs when build source is specified", () => {
      const { container } = customRender(
        <BuildLogAndRunLog
          level=""
          source={FilterSource.build}
          term={EMPTY_FILTER_TERM}
        />
      )
      expect(container.querySelectorAll(".LogLine")).toHaveLength(20)
      expect(screen.getAllByText(/Vigoda build line/)).toHaveLength(18)
      expect(screen.queryByText(/Vigoda pod line/)).toBeNull()
    })
  })

  describe("filters by level", () => {
    it("displays only warning logs when warning log level is specified", () => {
      const { container } = customRender(
        <BuildLogAndRunLog
          level={FilterLevel.warn}
          source=""
          term={EMPTY_FILTER_TERM}
        />
      )
      expect(container.querySelectorAll(".LogLine")).toHaveLength(
        2 * (1 + PROLOGUE_LENGTH)
      )
      const alerts = container.querySelectorAll(".is-endOfAlert")
      const lastAlert = alerts[alerts.length - 1]
      expect(lastAlert).toHaveTextContent("Vigoda pod warning line")
      expect(screen.queryByText(/Vigoda pod error line/)).toBeNull()
    })

    it("displays only error logs when error log level is specified", () => {
      const { container } = customRender(
        <BuildLogAndRunLog
          level={FilterLevel.error}
          source=""
          term={EMPTY_FILTER_TERM}
        />
      )

      expect(container.querySelectorAll(".LogLine")).toHaveLength(
        2 * (1 + PROLOGUE_LENGTH)
      )
      const alerts = container.querySelectorAll(".is-endOfAlert")
      const lastAlert = alerts[alerts.length - 1]
      expect(lastAlert).toHaveTextContent("Vigoda pod error line")
    })
  })

  describe("filters by term", () => {
    it("displays log lines that match the specified filter term", () => {
      const termWithResults = createFilterTermState("line 5")
      const { container } = customRender(
        <BuildLogAndRunLog source="" level="" term={termWithResults} />
      )

      expect(container.querySelectorAll(".LogLine")).toHaveLength(2)
      expect(screen.getAllByText(/line 5/)).toHaveLength(2)
      expect(screen.queryByText(/line 15/)).toBeNull()
    })

    it("displays zero log lines when no logs match the specified filter term", () => {
      const termWithResults = createFilterTermState("spaghetti")
      const { container } = customRender(
        <BuildLogAndRunLog source="" level="" term={termWithResults} />
      )

      expect(container.querySelectorAll(".LogLine")).toHaveLength(0)
    })
  })

  /**
   * The following tests rely on testing React component state directly,
   * which is not possible to do with React Testing Library.
   */

  describe("log rendering", () => {
    function getLogElements(container: HTMLElement) {
      return container.querySelectorAll(".LogLine")
    }

    const initLineCount = 2 * renderWindow

    let fakeRaf: TestRafContext
    let rootTree: Component<any>
    let container: HTMLDivElement
    let component: OverviewLogComponent

    beforeEach(() => {
      fakeRaf = newFakeRaf()

      class ManyLinesWrapper extends Component {
        render() {
          return (
            <MemoryRouter
              initialEntries={["/"]}
              future={{ v7_startTransition: true, v7_relativeSplatPath: true }}
            >
              <RafProvider value={fakeRaf}>
                <ManyLines count={initLineCount} />
              </RafProvider>
            </MemoryRouter>
          )
        }
      }

      const testHelpers = renderTestComponent(<ManyLinesWrapper />)
      rootTree = testHelpers.rootTree
      container = testHelpers.container
      component = findRenderedComponentWithType(rootTree, OverviewLogComponent)
    })

    it("engages autoscrolls on scroll down", () => {
      component.autoscroll = false
      component.scrollTop = 0
      component.rootRef.current.scrollTop = 1000
      component.onScroll()
      expect(component.scrollTop).toEqual(1000)

      // The scroll has been scheduled, but not engaged yet.
      expect(component.autoscrollRafId).toBeGreaterThan(0)
      expect(component.autoscroll).toEqual(false)

      fakeRaf.invoke(component.autoscrollRafId as number)
      expect(component.autoscroll).toEqual(true)
    })

    it("renders bottom logs first", () => {
      // Make sure no logs have been rendered yet.
      let getLogElements = () => container.querySelectorAll(".LogLine")

      expect(component.renderBufferRafId).toBeGreaterThan(0)
      expect(component.backwardBuffer.length).toEqual(initLineCount)
      expect(getLogElements().length).toEqual(0)

      // Invoke the RAF callback, and make sure that only a window's
      // worth of logs have been rendered.
      fakeRaf.invoke(component.renderBufferRafId as number)
      expect(component.backwardBuffer.length).toEqual(
        initLineCount - renderWindow
      )
      expect(getLogElements().length).toEqual(renderWindow)
      expect(getLogElements()[0].innerHTML).toEqual(
        expect.stringContaining(">line 250\n<")
      )

      // Invoke the RAF callback again, and make sure the remaining logs
      // were rendered.
      fakeRaf.invoke(component.renderBufferRafId as number)
      expect(component.backwardBuffer.length).toEqual(0)
      expect(getLogElements().length).toEqual(initLineCount)
      expect(getLogElements()[0].innerHTML).toEqual(
        expect.stringContaining(">line 0\n<")
      )

      // rendering is complete.
      expect(component.renderBufferRafId).toEqual(0)
    })

    it("renders new logs first", () => {
      expect(component.renderBufferRafId).toBeGreaterThan(0)
      expect(component.backwardBuffer.length).toEqual(initLineCount)
      expect(getLogElements(container).length).toEqual(0)

      // append new lines on top of the lines we already have.
      let newLineCount = 1.5 * renderWindow
      let lines = []
      for (let i = 0; i < newLineCount; i++) {
        lines.push(`incremental line ${i}\n`)
      }
      appendLines(component.props.logStore, "fe", ...lines)
      component.onLogUpdate({ action: LogUpdateAction.append })
      expect(component.forwardBuffer.length).toEqual(newLineCount)
      expect(component.backwardBuffer.length).toEqual(initLineCount)

      // Invoke the RAF callback, and make sure that new logs were rendered
      // and old logs were rendered.
      fakeRaf.invoke(component.renderBufferRafId as number)
      expect(component.forwardBuffer.length).toEqual(
        newLineCount - renderWindow
      )
      expect(component.backwardBuffer.length).toEqual(
        initLineCount - renderWindow
      )

      const logElements = getLogElements(container)
      expect(logElements.length).toEqual(initLineCount)
      expect(logElements[0].innerHTML).toEqual(
        expect.stringContaining(">line 250\n<")
      )
      expect(logElements[logElements.length - 1].innerHTML).toEqual(
        expect.stringContaining(">incremental line 249\n<")
      )

      // Invoke the RAF callback again, and make sure that new logs were rendered further up
      // and old logs were rendered further down.
      fakeRaf.invoke(component.renderBufferRafId as number)
      const logElementsAfterInvoke = getLogElements(container)
      expect(logElementsAfterInvoke[0].innerHTML).toEqual(
        expect.stringContaining(">line 0\n<")
      )
      expect(
        logElementsAfterInvoke[logElementsAfterInvoke.length - 1].innerHTML
      ).toEqual(expect.stringContaining(">incremental line 374\n<"))
    })
  })
})
