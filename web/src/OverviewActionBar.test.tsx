import { render, RenderOptions, screen, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
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
import { ButtonSet } from "./ApiButton"
import { EMPTY_FILTER_TERM, FilterLevel, FilterSource } from "./logfilters"
import OverviewActionBar, {
  createLogSearch,
  FILTER_INPUT_DEBOUNCE,
} from "./OverviewActionBar"
import { EmptyBar, FullBar } from "./OverviewActionBar.stories"
import { disableButton, oneResource, oneUIButton } from "./testdata"

const DEFAULT_FILTER_SET = {
  level: FilterLevel.all,
  source: FilterSource.all,
  term: EMPTY_FILTER_TERM,
}

function customRender(
  component: JSX.Element,
  wrapperProps: { history: MemoryHistory },
  options?: RenderOptions
) {
  return render(component, {
    wrapper: ({ children }) => (
      <Router history={wrapperProps.history}>
        <SnackbarProvider>{children}</SnackbarProvider>
      </Router>
    ),
    ...options,
  })
}

describe("OverviewActionBar", () => {
  let history: MemoryHistory
  beforeEach(() => {
    cleanupMockAnalyticsCalls()
    mockAnalyticsCalls()
    history = createMemoryHistory({ initialEntries: ["/"] })
  })

  afterEach(() => {
    cleanupMockAnalyticsCalls()
    jest.useRealTimers()
  })

  it("renders the top row with endpoints", () => {
    customRender(<FullBar />, { history })

    expect(
      screen.getByLabelText(/links and custom buttons/i)
    ).toBeInTheDocument()
    expect(screen.getAllByRole("link")).toHaveLength(2)
  })

  it("renders the top row with pod ID", () => {
    customRender(<FullBar />, { history })

    expect(
      screen.getByLabelText(/links and custom buttons/i)
    ).toBeInTheDocument()
    expect(screen.getAllByRole("button", { name: /Pod ID/i })).toHaveLength(1)
  })

  it("does NOT render the top row when there are no endpoints, pods, or buttons", () => {
    customRender(<EmptyBar />, { history })

    expect(screen.queryByLabelText(/links and custom buttons/i)).toBeNull()
  })

  describe("log filters", () => {
    beforeEach(() => customRender(<FullBar />, { history }))

    it("navigates to warning filter when warning log filter button is clicked", () => {
      userEvent.click(screen.getByRole("button", { name: /warnings/i }))

      expect(history.location.search).toEqual("?level=warn&source=")

      expectIncrs({
        name: "ui.web.filterLevel",
        tags: { action: AnalyticsAction.Click, level: "warn", source: "" },
      })
    })

    it("navigates to build warning filter when both building and warning log filter buttons are clicked", () => {
      userEvent.click(
        screen.getByRole("button", { name: "Select warn log sources" })
      )
      userEvent.click(screen.getByRole("menuitem", { name: /build/i }))

      expect(history.location.search).toEqual("?level=warn&source=build")

      expectIncrs({
        name: "ui.web.filterSourceMenu",
        tags: { action: AnalyticsAction.Click },
      })
    })
  })

  describe("disabled resource view", () => {
    beforeEach(() => {
      const resource = oneResource({ name: "not-enabled", disabled: true })
      const buttonSet: ButtonSet = {
        default: [
          oneUIButton({ componentID: "not-enabled", buttonText: "Click me" }),
        ],
        toggleDisable: disableButton("not-enabled", false),
      }
      customRender(
        <OverviewActionBar
          resource={resource}
          filterSet={DEFAULT_FILTER_SET}
          buttons={buttonSet}
        />,
        { history }
      )
    })

    it("should display the disable toggle button", () => {
      expect(
        screen.getByRole("button", { name: /trigger enable resource/i })
      ).toBeInTheDocument()
    })

    it("should NOT display any `default` custom buttons", () => {
      expect(screen.queryByRole("button", { name: /click me/i })).toBeNull()
    })

    it("should NOT display the filter menu", () => {
      expect(screen.queryByRole("button", { name: /warnings/i })).toBeNull()
      expect(screen.queryByRole("button", { name: /errors/i })).toBeNull()
      expect(screen.queryByRole("button", { name: /all levels/i })).toBeNull()
      expect(screen.queryByRole("textbox")).toBeNull()
    })

    it("should NOT display endpoint information", () => {
      expect(screen.queryAllByRole("link")).toHaveLength(0)
    })

    it("should NOT display podId information", () => {
      expect(screen.queryAllByRole("button", { name: /Pod ID/i })).toHaveLength(
        0
      )
    })
  })

  describe("custom buttons", () => {
    const customButtons = [
      oneUIButton({ componentID: "vigoda", disabled: true }),
    ]
    const toggleDisable = disableButton("vigoda", true)
    beforeEach(() => {
      customRender(
        <OverviewActionBar
          filterSet={DEFAULT_FILTER_SET}
          buttons={{ default: customButtons, toggleDisable }}
        />,
        { history }
      )
    })
    it("disables a button that should be disabled", () => {
      const disabledButton = screen.getByLabelText(
        `Trigger ${customButtons[0].spec?.text}`
      )
      expect(disabledButton).toBeDisabled()
    })

    it("renders disable-resource buttons separately from other buttons", () => {
      const topRow = screen.getByLabelText(/links and custom buttons/i)
      const buttonsInTopRow = within(topRow).getAllByRole("button")
      const toggleDisableButton = screen.getByLabelText(
        "Trigger Disable Resource"
      )

      expect(buttonsInTopRow).toHaveLength(1)
      expect(toggleDisableButton).toBeInTheDocument()
      expect(
        within(topRow).queryByLabelText("Trigger Disable Resource")
      ).toBeNull()
    })
  })

  describe("term filter input", () => {
    it("renders with no initial value if there is no existing term filter", () => {
      customRender(<FullBar />, { history })

      expect(
        screen.getByRole("textbox", { name: /filter resource logs/i })
      ).toHaveValue("")
    })

    it("renders with an initial value if there is an existing term filter", () => {
      history.push({
        pathname: "/",
        search: createLogSearch("", { term: "bleep bloop" }).toString(),
      })

      customRender(<FullBar />, { history })

      expect(
        screen.getByRole("textbox", { name: /filter resource logs/i })
      ).toHaveValue("bleep bloop")
    })

    it("changes the global term filter state when its value changes", () => {
      jest.useFakeTimers()

      customRender(<FullBar />, { history })

      userEvent.type(screen.getByRole("textbox"), "docker")

      jest.advanceTimersByTime(FILTER_INPUT_DEBOUNCE)

      expect(history.location.search.toString()).toEqual("?term=docker")
    })

    it("uses debouncing to update the global term filter state", () => {
      jest.useFakeTimers()

      customRender(<FullBar />, { history })

      userEvent.type(screen.getByRole("textbox"), "doc")

      jest.advanceTimersByTime(FILTER_INPUT_DEBOUNCE / 2)

      // The debouncing time hasn't passed yet, so we don't expect to see any changes
      expect(history.location.search.toString()).toEqual("")

      userEvent.type(screen.getByRole("textbox"), "ker")

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

      customRender(<FullBar />, { history })

      userEvent.type(screen.getByRole("textbox"), "help")

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
})
