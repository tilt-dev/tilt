import { render, RenderOptions, screen } from "@testing-library/react"
import { createRef } from "react"
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
} from "./OverviewLogPane.stories"
import { newFakeRaf, RafProvider, SyncRafProvider, TestRafContext } from "./raf"
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
    let container: HTMLDivElement
    let component: OverviewLogComponent

    beforeEach(() => {
      fakeRaf = newFakeRaf()

      const logRef = createRef<OverviewLogComponent>()
      const rendered = render(
        <MemoryRouter
          initialEntries={["/"]}
          future={{ v7_startTransition: true, v7_relativeSplatPath: true }}
        >
          <RafProvider value={fakeRaf}>
            <ManyLines count={initLineCount} innerRef={logRef} />
          </RafProvider>
        </MemoryRouter>
      )
      container = rendered.container as HTMLDivElement
      component = logRef.current!
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

    // https://github.com/tilt-dev/tilt/issues/6096
    describe("truncation", () => {
      // Renders all the initial lines, then truncates the log store the same
      // way a log append does in production: the appended segment lands in
      // the store, ensureMaxLength() truncates it, and then the deferred
      // append and truncate events are delivered.
      function renderAllThenTruncate() {
        fakeRaf.invoke(component.renderBufferRafId as number)
        fakeRaf.invoke(component.renderBufferRafId as number)
        expect(getLogElements(container).length).toEqual(initLineCount)

        const logStore = component.props.logStore
        logStore.maxLogLength = 2000
        appendLines(logStore, "fe", "new line A\n")
        component.onLogUpdate({ action: LogUpdateAction.append })
        component.onLogUpdate({ action: LogUpdateAction.truncate })

        return logStore.manifestLog("fe")
      }

      it("keeps rendered lines and remaps their indices", () => {
        const lastEl = container.querySelector(
          '[data-sl-index="499"]'
        ) as Element | null
        expect(lastEl).toBeNull() // nothing rendered yet

        const survivors = renderAllThenTruncate()
        expect(survivors.length).toBeGreaterThan(0)
        expect(survivors.length).toBeLessThan(initLineCount)

        // The pane never went blank: all the surviving lines except the
        // fresh "new line A" are still rendered, without waiting for an
        // animation frame.
        const els = getLogElements(container)
        expect(els.length).toEqual(survivors.length - 1)

        // The oldest lines were trimmed.
        expect(els[0].innerHTML).toEqual(
          expect.stringContaining(`>${survivors[0].text}\n<`)
        )

        // The rendered elements were re-used, not rebuilt, and their
        // data-sl-index attributes were remapped.
        const newLast = survivors.find((line) => line.text === "line 499")!
        const lastElAfter = container.querySelector(
          `[data-sl-index="${newLast.storedLineIndex}"]`
        )!
        expect(lastElAfter.innerHTML).toEqual(
          expect.stringContaining(">line 499\n<")
        )

        // The fresh line renders on the next animation frame.
        expect(component.forwardBuffer.map((l) => l.text)).toEqual([
          "new line A",
        ])
        fakeRaf.invoke(component.renderBufferRafId as number)
        const elsAfter = getLogElements(container)
        expect(elsAfter.length).toEqual(survivors.length)
        expect(elsAfter[elsAfter.length - 1].innerHTML).toEqual(
          expect.stringContaining(">new line A\n<")
        )
      })

      it("re-uses the same DOM nodes across truncation", () => {
        fakeRaf.invoke(component.renderBufferRafId as number)
        fakeRaf.invoke(component.renderBufferRafId as number)
        const lastEl = container.querySelector('[data-sl-index="499"]')!

        const logStore = component.props.logStore
        logStore.maxLogLength = 2000
        appendLines(logStore, "fe", "new line A\n")
        component.onLogUpdate({ action: LogUpdateAction.append })
        component.onLogUpdate({ action: LogUpdateAction.truncate })

        // The same DOM node is still attached, under its new index.
        expect(container.contains(lastEl)).toEqual(true)
        const survivors = logStore.manifestLog("fe")
        const newLast = survivors.find((line) => line.text === "line 499")!
        expect(lastEl.getAttribute("data-sl-index")).toEqual(
          String(newLast.storedLineIndex)
        )
      })

      it("preserves the autoscroll state", () => {
        renderAllThenTruncate()
        expect(component.autoscroll).toEqual(true)
        expect(component.scrollTop).toEqual(-1)
      })

      it("does not re-engage autoscroll when scrolled up", () => {
        fakeRaf.invoke(component.renderBufferRafId as number)
        fakeRaf.invoke(component.renderBufferRafId as number)

        component.autoscroll = false
        component.scrollTop = 1000

        const logStore = component.props.logStore
        logStore.maxLogLength = 2000
        appendLines(logStore, "fe", "new line A\n")
        component.onLogUpdate({ action: LogUpdateAction.append })
        component.onLogUpdate({ action: LogUpdateAction.truncate })

        expect(component.autoscroll).toEqual(false)
        expect(component.scrollTop).toEqual(-1)

        // The surviving lines are still rendered.
        const survivors = logStore.manifestLog("fe")
        expect(getLogElements(container).length).toEqual(survivors.length - 1)
      })

      it("renders incremental appends after truncation", () => {
        const survivors = renderAllThenTruncate()
        fakeRaf.invoke(component.renderBufferRafId as number)
        expect(getLogElements(container).length).toEqual(survivors.length)

        appendLines(component.props.logStore, "fe", "post 0\n", "post 1\n")
        component.onLogUpdate({ action: LogUpdateAction.append })
        expect(component.forwardBuffer.map((l) => l.text)).toEqual([
          "post 0",
          "post 1",
        ])

        fakeRaf.invoke(component.renderBufferRafId as number)
        const els = getLogElements(container)
        expect(els.length).toEqual(survivors.length + 2)
        expect(els[els.length - 1].innerHTML).toEqual(
          expect.stringContaining(">post 1\n<")
        )
      })

      it("falls back to a full re-render when the view is removed", () => {
        fakeRaf.invoke(component.renderBufferRafId as number)
        fakeRaf.invoke(component.renderBufferRafId as number)
        expect(getLogElements(container).length).toEqual(initLineCount)

        const logStore = component.props.logStore
        logStore.removeSpans(["fe"])
        component.onLogUpdate({ action: LogUpdateAction.truncate })

        expect(getLogElements(container).length).toEqual(0)
      })
    })
  })
})
