import { render, RenderOptions, screen } from "@testing-library/react"
import { mount } from "enzyme"
import { MemoryRouter } from "react-router"
import {
  createFilterTermState,
  EMPTY_FILTER_TERM,
  FilterLevel,
  FilterSource,
} from "./logfilters"
import { LogUpdateAction } from "./LogStore"
import {
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
} from "./OverviewLogPane.stories"
import { newFakeRaf, RafProvider, SyncRafProvider } from "./raf"
import { appendLines } from "./testlogs"

function customRender(component: JSX.Element, options?: RenderOptions) {
  return render(component, {
    wrapper: ({ children }) => (
      <MemoryRouter initialEntries={["/"]}>
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

  it("escapes html and linkifies", () => {
    customRender(<StyledLines />)
    expect(screen.getAllByRole("link")).toHaveLength(2)
    expect(screen.queryByRole("button")).toBeNull()
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
})

/**
 * The following tests rely on testing React component state directly,
 * which is not possible to do with React Testing Library. They'll need to
 * either use React's test utilities (which involve some funky type manipulation)
 * or perhaps modify/wrap the component to render its state to the DOM instead.
 */

it("engages autoscrolls on scroll down", () => {
  let fakeRaf = newFakeRaf()
  let root = mount(
    <MemoryRouter initialEntries={["/"]}>
      <RafProvider value={fakeRaf}>
        <ManyLines count={100} />
      </RafProvider>
    </MemoryRouter>
  )
  let component = root
    .find(OverviewLogComponent)
    .instance() as OverviewLogComponent

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
  let fakeRaf = newFakeRaf()
  let lineCount = 2 * renderWindow
  let root = mount(
    <MemoryRouter initialEntries={["/"]}>
      <RafProvider value={fakeRaf}>
        <ManyLines count={lineCount} />
      </RafProvider>
    </MemoryRouter>
  )

  // Make sure no logs have been rendered yet.
  let rootEl = root.getDOMNode()
  let lineEls = () => rootEl.querySelectorAll(".LogLine")
  let component = root
    .find(OverviewLogComponent)
    .instance() as OverviewLogComponent
  expect(component.renderBufferRafId).toBeGreaterThan(0)
  expect(component.backwardBuffer.length).toEqual(lineCount)
  expect(lineEls().length).toEqual(0)

  // Invoke the RAF callback, and make sure that only a window's
  // worth of logs have been rendered.
  fakeRaf.invoke(component.renderBufferRafId as number)
  expect(component.backwardBuffer.length).toEqual(lineCount - renderWindow)
  expect(lineEls().length).toEqual(renderWindow)
  expect(lineEls()[0].innerHTML).toEqual(
    expect.stringContaining(">line 250\n<")
  )

  // Invoke the RAF callback again, and make sure the remaining logs
  // were rendered.
  fakeRaf.invoke(component.renderBufferRafId as number)
  expect(component.backwardBuffer.length).toEqual(0)
  expect(lineEls().length).toEqual(lineCount)
  expect(lineEls()[0].innerHTML).toEqual(expect.stringContaining(">line 0\n<"))

  // rendering is complete.
  expect(component.renderBufferRafId).toEqual(0)
})

it("renders new logs first", () => {
  let fakeRaf = newFakeRaf()
  let initLineCount = 2 * renderWindow
  let root = mount(
    <MemoryRouter initialEntries={["/"]}>
      <RafProvider value={fakeRaf}>
        <ManyLines count={initLineCount} />
      </RafProvider>
    </MemoryRouter>
  )

  let rootEl = root.getDOMNode()
  let lineEls = () => rootEl.querySelectorAll(".LogLine")
  let component = root
    .find(OverviewLogComponent)
    .instance() as OverviewLogComponent
  expect(component.renderBufferRafId).toBeGreaterThan(0)
  expect(component.backwardBuffer.length).toEqual(initLineCount)
  expect(lineEls().length).toEqual(0)

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
  expect(component.forwardBuffer.length).toEqual(newLineCount - renderWindow)
  expect(component.backwardBuffer.length).toEqual(initLineCount - renderWindow)
  expect(lineEls().length).toEqual(renderWindow * 2)
  expect(lineEls()[0].innerHTML).toEqual(
    expect.stringContaining(">line 250\n<")
  )
  expect(lineEls()[lineEls().length - 1].innerHTML).toEqual(
    expect.stringContaining(">incremental line 249\n<")
  )

  // Invoke the RAF callback again, and make sure that new logs were rendered further up
  // and old logs were rendered further down.
  fakeRaf.invoke(component.renderBufferRafId as number)
  expect(lineEls()[0].innerHTML).toEqual(expect.stringContaining(">line 0\n<"))
  expect(lineEls()[lineEls().length - 1].innerHTML).toEqual(
    expect.stringContaining(">incremental line 374\n<")
  )
})
