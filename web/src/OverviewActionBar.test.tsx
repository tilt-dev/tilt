import MenuItem from "@material-ui/core/MenuItem"
import { mount, ReactWrapper } from "enzyme"
import { createMemoryHistory, MemoryHistory } from "history"
import { SnackbarProvider } from "notistack"
import React from "react"
import { Router } from "react-router"
import { AnalyticsAction } from "./analytics"
import {
  cleanupMockAnalyticsCalls,
  expectIncrs,
  mockAnalyticsCalls,
} from "./analytics_test_helpers"
import { ApiButton, ButtonSet } from "./ApiButton"
import { InstrumentedButton } from "./instrumentedComponents"
import LogActions from "./LogActions"
import { EMPTY_FILTER_TERM, FilterLevel, FilterSource } from "./logfilters"
import OverviewActionBar, {
  ActionBarBottomRow,
  ActionBarTopRow,
  ButtonLeftPill,
  CopyButton,
  createLogSearch,
  Endpoint,
  FilterRadioButton,
  FilterTermField,
  FILTER_FIELD_ID,
  FILTER_INPUT_DEBOUNCE,
} from "./OverviewActionBar"
import { EmptyBar, FullBar } from "./OverviewActionBar.stories"
import { disableButton, oneResource, oneUIButton } from "./testdata"

let history: MemoryHistory
beforeEach(() => {
  mockAnalyticsCalls()
  history = createMemoryHistory()
  history.push({ pathname: "/" })
})

afterEach(() => {
  cleanupMockAnalyticsCalls()
  jest.useRealTimers()
})

function mountBar(e: JSX.Element) {
  return mount(
    <Router history={history}>
      <SnackbarProvider>{e}</SnackbarProvider>
    </Router>
  )
}

it("shows endpoints", () => {
  let root = mountBar(<FullBar />)
  let topBar = root.find(ActionBarTopRow)
  expect(topBar).toHaveLength(1)

  let endpoints = topBar.find(Endpoint)
  expect(endpoints).toHaveLength(2)
})

it("shows pod ID", () => {
  const root = mountBar(<FullBar />)
  const podId = root.find(ActionBarTopRow).find(CopyButton)

  expect(podId).toHaveLength(1)
  expect(podId.text()).toContain("my-deadbeef-pod Pod ID") // Hardcoded from test data
})

it("skips the top bar when empty", () => {
  let root = mountBar(<EmptyBar />)
  let topBar = root.find(ActionBarTopRow)
  expect(topBar).toHaveLength(0)
})

it("navigates to warning filter", () => {
  let root = mountBar(<FullBar />)
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
  let root = mountBar(<FullBar />)
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

describe("disabled resource view", () => {
  let root: ReactWrapper<any, any>

  beforeEach(() => {
    const resource = oneResource({
      name: "i-am-not-enabled",
      disabled: true,
    })
    const filterSet = {
      level: FilterLevel.all,
      source: FilterSource.all,
      term: EMPTY_FILTER_TERM,
    }
    const buttonSet: ButtonSet = {
      default: [oneUIButton({ componentID: "i-am-not-enabled" })],
      toggleDisable: disableButton("i-am-not-enabled", false),
    }

    root = mountBar(
      <OverviewActionBar
        resource={resource}
        filterSet={filterSet}
        buttons={buttonSet}
      />
    )
  })

  it("should display the disable toggle button", () => {
    const bottomRowButtons = root.find(ActionBarBottomRow).find(ApiButton)
    expect(bottomRowButtons.length).toBeGreaterThanOrEqual(1)
    expect(
      bottomRowButtons.at(bottomRowButtons.length - 1).prop("uiButton").metadata
        ?.name
    ).toEqual("toggle-i-am-not-enabled-disable")
  })

  it("should NOT display any `default` custom buttons", () => {
    const topRowButtons = root.find(ActionBarTopRow).find(ApiButton)
    expect(topRowButtons.length).toBe(0)
  })

  it("should NOT display the filter menu", () => {
    const bottomRow = root.find(ActionBarBottomRow)
    const filterButtons = bottomRow.find(FilterRadioButton)
    const filterTermField = bottomRow.find(FilterTermField)
    const logActionsMenu = bottomRow.find(LogActions)

    expect(filterButtons).toHaveLength(0)
    expect(filterTermField).toHaveLength(0)
    expect(logActionsMenu).toHaveLength(0)
  })

  it("should NOT display endpoint information", () => {
    const endpoints = root.find(ActionBarTopRow).find(Endpoint)
    expect(endpoints).toHaveLength(0)
  })

  it("should NOT display podId information", () => {
    const podInfo = root.find(ActionBarTopRow).find(CopyButton)
    expect(podInfo).toHaveLength(0)
  })
})

describe("buttons", () => {
  it("shows endpoint buttons", () => {
    let root = mountBar(<FullBar />)
    let topBar = root.find(ActionBarTopRow)
    expect(topBar).toHaveLength(1)

    let endpoints = topBar.find(Endpoint)
    expect(endpoints).toHaveLength(2)
  })

  it("disables a button that should be disabled", () => {
    let uiButtons = [oneUIButton({ componentID: "vigoda", disabled: true })]
    let filterSet = {
      level: FilterLevel.all,
      source: FilterSource.all,
      term: EMPTY_FILTER_TERM,
    }
    let root = mountBar(
      <OverviewActionBar
        filterSet={filterSet}
        buttons={{ default: uiButtons }}
      />
    )
    let topBar = root.find(ActionBarTopRow)
    let buttons = topBar.find(InstrumentedButton)
    expect(buttons).toHaveLength(1)
    expect(buttons.at(0).prop("disabled")).toBe(true)
  })

  it("renders disable-resource buttons separately from other buttons", () => {
    const root = mountBar(<FullBar />)

    const topRowButtons = root.find(ActionBarTopRow).find(ApiButton)
    expect(topRowButtons).toHaveLength(1)
    expect(topRowButtons.at(0).prop("uiButton").metadata?.name).toEqual(
      "button2"
    )

    const bottomRowButtons = root.find(ActionBarBottomRow).find(ApiButton)
    expect(bottomRowButtons.length).toBeGreaterThanOrEqual(1)
    expect(
      bottomRowButtons.at(bottomRowButtons.length - 1).prop("uiButton").metadata
        ?.name
    ).toEqual("toggle-vigoda-disable")
  })
})

describe("term filter input", () => {
  const FILTER_INPUT = `input#${FILTER_FIELD_ID}`
  let root: ReactWrapper<any, Readonly<{}>, React.Component<{}, {}, any>>

  it("renders with no initial value if there is no existing term filter", () => {
    history.push({
      pathname: "/",
      search: createLogSearch("", {}).toString(),
    })

    root = mountBar(<FullBar />)

    const inputField = root.find(FILTER_INPUT)
    expect(inputField.props().value).toBe("")
  })

  it("renders with an initial value if there is an existing term filter", () => {
    history.push({
      pathname: "/",
      search: createLogSearch("", { term: "bleep bloop" }).toString(),
    })

    root = mountBar(<FullBar />)

    const inputField = root.find(FILTER_INPUT)
    expect(inputField.props().value).toBe("bleep bloop")
  })

  it("changes the global term filter state when its value changes", () => {
    jest.useFakeTimers()

    root = mountBar(<FullBar />)

    const inputField = root.find(FILTER_INPUT)
    inputField.simulate("change", { target: { value: "docker" } })

    jest.advanceTimersByTime(FILTER_INPUT_DEBOUNCE)

    expect(history.location.search.toString()).toEqual("?term=docker")
  })

  it("uses debouncing to update the global term filter state", () => {
    jest.useFakeTimers()

    root = mountBar(<FullBar />)

    const inputField = root.find(FILTER_INPUT)
    inputField.simulate("change", { target: { value: "doc" } })

    jest.advanceTimersByTime(FILTER_INPUT_DEBOUNCE / 2)

    // The debouncing time hasn't passed yet, so we don't expect to see any changes
    expect(history.location.search.toString()).toEqual("")

    inputField.simulate("change", { target: { value: "docker" } })

    // The debouncing time hasn't passed yet, so we don't expect to see any changes
    expect(history.location.search.toString()).toEqual("")

    jest.advanceTimersByTime(FILTER_INPUT_DEBOUNCE)

    // Since the debouncing time has passed, we expect to see the final
    // change reflected
    expect(history.location.search.toString()).toEqual("?term=docker")
  })

  it("retains any current level and source filters when its value changes", () => {
    jest.useFakeTimers()

    history.push({ pathname: "/", search: "level=warn&source=build" })

    root = mountBar(<FullBar />)

    const inputField = root.find(FILTER_INPUT)
    inputField.simulate("change", { target: { value: "help" } })

    jest.advanceTimersByTime(FILTER_INPUT_DEBOUNCE)

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
