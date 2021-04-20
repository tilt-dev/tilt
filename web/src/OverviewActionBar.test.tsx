import MenuItem from "@material-ui/core/MenuItem"
import { mount } from "enzyme"
import { createMemoryHistory } from "history"
import { MemoryRouter, Router } from "react-router"
import {
  cleanupMockAnalyticsCalls,
  expectIncrs,
  mockAnalyticsCalls,
} from "./analytics_test_helpers"
import { FilterLevel, FilterSource } from "./logfilters"
import {
  ActionBarTopRow,
  ButtonLeftPill,
  Endpoint,
  FilterRadioButton,
} from "./OverviewActionBar"
import { EmptyBar, FullBar } from "./OverviewActionBar.stories"

beforeEach(() => {
  mockAnalyticsCalls()
})

afterEach(() => {
  cleanupMockAnalyticsCalls()
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
    tags: { action: "click", level: "warn", source: "" },
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
  expect(history.location.search).toEqual("?source=build&level=warn")
})
