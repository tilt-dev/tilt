import MenuItem from "@material-ui/core/MenuItem"
import { mount, ReactWrapper } from "enzyme"
import { createMemoryHistory, MemoryHistory } from "history"
import { MemoryRouter, Router } from "react-router"
import { AnalyticsAction } from "./analytics"
import {
  cleanupMockAnalyticsCalls,
  expectIncrs,
  mockAnalyticsCalls,
} from "./analytics_test_helpers"
import { InstrumentedButton } from "./instrumentedComponents"
import { EMPTY_FILTER_TERM, FilterLevel, FilterSource } from "./logfilters"
import OverviewActionBar, {
  ActionBarTopRow,
  ButtonLeftPill,
  createLogSearch,
  Endpoint,
  FilterRadioButton,
  FILTER_FIELD_ID,
  FILTER_INPUT_DEBOUNCE,
} from "./OverviewActionBar"
import { EmptyBar, FullBar } from "./OverviewActionBar.stories"
import { oneButton } from "./testdata"

beforeEach(() => {
  mockAnalyticsCalls()
})

afterEach(() => {
  cleanupMockAnalyticsCalls()
  jest.useRealTimers()
})

it("shows endpoints", () => {
  let root = mount(
    <MemoryRouter initialEntries={["/"]}>
      <FullBar />
    </MemoryRouter>
  )
  let topBar = root.find(ActionBarTopRow)
  expect(topBar).toHaveLength(1)

  let endpoints = topBar.find(Endpoint)
  expect(endpoints).toHaveLength(2)
})

it("skips the top bar when empty", () => {
  let root = mount(
    <MemoryRouter initialEntries={["/"]}>
      <EmptyBar />
    </MemoryRouter>
  )
  let topBar = root.find(ActionBarTopRow)
  expect(topBar).toHaveLength(0)
})

it("navigates to warning filter", () => {
  let history = createMemoryHistory()
  let root = mount(
    <Router history={history}>
      <FullBar />
    </Router>
  )
  let warnFilter = root
    .find(FilterRadioButton)
    .filter({ level: FilterLevel.warn })
  expect(warnFilter).toHaveLength(1)
  let leftButton = warnFilter.find(ButtonLeftPill)
  expect(leftButton).toHaveLength(1)
  leftButton.simulate("click")
  expect(history.location.search).toEqual("?level=warn&source=")

  expectIncrs({
    name: "ui.web.filterLevel",
    tags: { action: AnalyticsAction.Click, level: "warn", source: "" },
  })
})

it("navigates to build warning filter", () => {
  let history = createMemoryHistory()
  let root = mount(
    <Router history={history}>
      <FullBar />
    </Router>
  )
  let warnFilter = root
    .find(FilterRadioButton)
    .filter({ level: FilterLevel.warn })
  expect(warnFilter).toHaveLength(1)
  let sourceItems = warnFilter.find(MenuItem)
  expect(sourceItems).toHaveLength(3)
  let buildItem = sourceItems.filter({ "data-filter": FilterSource.build })
  expect(buildItem).toHaveLength(1)
  buildItem.simulate("click")
  expect(history.location.search).toEqual("?level=warn&source=build")
})

describe("buttons", () => {
  it("shows endpoint buttons", () => {
    let root = mount(
      <MemoryRouter initialEntries={["/"]}>
        <FullBar />
      </MemoryRouter>
    )
    let topBar = root.find(ActionBarTopRow)
    expect(topBar).toHaveLength(1)

    let endpoints = topBar.find(Endpoint)
    expect(endpoints).toHaveLength(2)
  })

  it("disables disabled buttons", () => {
    let uiButtons = [oneButton(1, "vigoda")]
    uiButtons[0].spec!.disabled = true
    let filterSet = {
      level: FilterLevel.all,
      source: FilterSource.all,
      term: EMPTY_FILTER_TERM,
    }
    let root = mount(
      <MemoryRouter initialEntries={["/"]}>
        <OverviewActionBar filterSet={filterSet} buttons={uiButtons} />
      </MemoryRouter>
    )
    let topBar = root.find(ActionBarTopRow)
    let buttons = topBar.find(InstrumentedButton)
    expect(buttons).toHaveLength(1)
    expect(buttons.at(0).prop("disabled")).toBe(true)
  })
})

describe("Term filter input", () => {
  const FILTER_INPUT = `input#${FILTER_FIELD_ID}`
  let history: MemoryHistory
  let root: ReactWrapper<any, Readonly<{}>, React.Component<{}, {}, any>>

  beforeEach(() => (history = createMemoryHistory()))

  it("renders with no initial value if there is no existing term filter", () => {
    history.push({
      pathname: "/",
      search: createLogSearch("", {}).toString(),
    })

    root = mount(
      <Router history={history}>
        <FullBar />
      </Router>
    )

    const inputField = root.find(FILTER_INPUT)
    expect(inputField.props().value).toBe("")
  })

  it("renders with an initial value if there is an existing term filter", () => {
    history.push({
      pathname: "/",
      search: createLogSearch("", { term: "bleep bloop" }).toString(),
    })

    root = mount(
      <Router history={history}>
        <FullBar />
      </Router>
    )

    const inputField = root.find(FILTER_INPUT)
    expect(inputField.props().value).toBe("bleep bloop")
  })

  it("changes the global term filter state when its value changes", () => {
    jest.useFakeTimers()

    root = mount(
      <Router history={history}>
        <FullBar />
      </Router>
    )

    const inputField = root.find(FILTER_INPUT)
    inputField.simulate("change", { target: { value: "docker" } })

    jest.runTimersToTime(FILTER_INPUT_DEBOUNCE)

    expect(history.location.search.toString()).toEqual("?term=docker")
  })

  it("uses debouncing to update the global term filter state", () => {
    jest.useFakeTimers()

    root = mount(
      <Router history={history}>
        <FullBar />
      </Router>
    )

    const inputField = root.find(FILTER_INPUT)
    inputField.simulate("change", { target: { value: "doc" } })

    jest.runTimersToTime(FILTER_INPUT_DEBOUNCE / 2)

    // The debouncing time hasn't passed yet, so we don't expect to see any changes
    expect(history.location.search.toString()).toEqual("")

    inputField.simulate("change", { target: { value: "docker" } })

    // The debouncing time hasn't passed yet, so we don't expect to see any changes
    expect(history.location.search.toString()).toEqual("")

    jest.runTimersToTime(FILTER_INPUT_DEBOUNCE)

    // Since the debouncing time has passed, we expect to see the final
    // change reflected
    expect(history.location.search.toString()).toEqual("?term=docker")
  })

  it("retains any current level and source filters when its value changes", () => {
    jest.useFakeTimers()

    history.push({ pathname: "/", search: "level=warn&source=build" })

    root = mount(
      <Router history={history}>
        <FullBar />
      </Router>
    )

    const inputField = root.find(FILTER_INPUT)
    inputField.simulate("change", { target: { value: "help" } })

    jest.runTimersToTime(FILTER_INPUT_DEBOUNCE)

    expect(history.location.search.toString()).toEqual(
      "?level=warn&source=build&term=help"
    )
  })
})

describe("createLogSearch", () => {
  let currentSearch: URLSearchParams
  beforeEach(() => (currentSearch = new URLSearchParams()))

  it("sets the params that are passed in", () => {
    expect(
      createLogSearch(currentSearch.toString(), {
        level: FilterLevel.all,
        term: "find me",
        source: FilterSource.build,
      }).toString()
    ).toBe("level=&source=build&term=find+me")

    expect(
      createLogSearch(currentSearch.toString(), {
        level: FilterLevel.warn,
      }).toString()
    ).toBe("level=warn")

    expect(
      createLogSearch(currentSearch.toString(), {
        term: "",
        source: FilterSource.runtime,
      }).toString()
    ).toBe("source=runtime&term=")
  })

  it("overrides params if a new value is defined", () => {
    currentSearch.set("level", FilterLevel.warn)
    expect(
      createLogSearch(currentSearch.toString(), {
        level: FilterLevel.error,
      }).toString()
    ).toBe("level=error")
    currentSearch.delete("level")

    currentSearch.set("level", "a meaningless value")
    currentSearch.set("term", "")
    expect(
      createLogSearch(currentSearch.toString(), {
        level: FilterLevel.all,
        term: "service",
      }).toString()
    ).toBe("level=&term=service")
  })

  it("preserves existing params if no new value is defined", () => {
    currentSearch.set("source", FilterSource.build)
    expect(
      createLogSearch(currentSearch.toString(), {
        term: "test",
      }).toString()
    ).toBe("source=build&term=test")
  })
})
