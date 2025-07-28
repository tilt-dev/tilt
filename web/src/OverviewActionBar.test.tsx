import {
  act,
  render,
  RenderOptions,
  screen,
  within,
} from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router"
import { useLocation } from "react-router-dom"
import { SnackbarProvider } from "notistack"
import React from "react"
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

let location: any = window.location

function LocationCapture() {
  location = useLocation()
  return null
}

// Helper to extract search params from the rendered MemoryRouter
function getSearch() {
  return location.search
}

function customRender(
  component: JSX.Element,
  wrapperProps: { initialEntries?: string[] } = {},
  options?: RenderOptions
) {
  return render(component, {
    wrapper: ({ children }) => (
      <MemoryRouter initialEntries={wrapperProps.initialEntries || ["/"]}>
        <SnackbarProvider>{children}</SnackbarProvider>
        <LocationCapture />
      </MemoryRouter>
    ),
    ...options,
  })
}

describe("OverviewActionBar", () => {
  it("renders the top row with endpoints", () => {
    customRender(<FullBar />)

    expect(
      screen.getByLabelText(/links and custom buttons/i)
    ).toBeInTheDocument()
    expect(screen.getAllByRole("link")).toHaveLength(2)
  })

  it("renders the top row with pod ID", () => {
    customRender(<FullBar />)

    expect(
      screen.getByLabelText(/links and custom buttons/i)
    ).toBeInTheDocument()
    expect(screen.getAllByRole("button", { name: /Pod ID/i })).toHaveLength(1)
  })

  it("does NOT render the top row when there are no endpoints, pods, or buttons", () => {
    customRender(<EmptyBar />)

    expect(screen.queryByLabelText(/links and custom buttons/i)).toBeNull()
  })

  describe("log filters", () => {
    beforeEach(() => customRender(<FullBar />))

    it("navigates to warning filter when warning log filter button is clicked", () => {
      userEvent.click(screen.getByRole("button", { name: /warnings/i }))

      expect(getSearch()).toEqual("?level=warn")
    })

    it("navigates to build warning filter when both building and warning log filter buttons are clicked", () => {
      userEvent.click(
        screen.getByRole("button", { name: "Select warn log sources" })
      )
      userEvent.click(screen.getByRole("menuitem", { name: /build/i }))

      expect(getSearch()).toEqual("?level=warn&source=build")
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
        />
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
        />
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
      customRender(<FullBar />)

      expect(
        screen.getByRole("textbox", { name: /filter resource logs/i })
      ).toHaveValue("")
    })

    it("renders with an initial value if there is an existing term filter", () => {
      customRender(<FullBar />, { initialEntries: ["/?term=bleep+bloop"] })

      expect(
        screen.getByRole("textbox", { name: /filter resource logs/i })
      ).toHaveValue("bleep bloop")
    })

    it("changes the global term filter state when its value changes", () => {
      jest.useFakeTimers()

      customRender(<FullBar />)

      userEvent.type(screen.getByRole("textbox"), "docker")

      jest.advanceTimersByTime(FILTER_INPUT_DEBOUNCE)

      expect(getSearch()).toEqual("?term=docker")
    })

    it("uses debouncing to update the global term filter state", () => {
      jest.useFakeTimers()

      customRender(<FullBar />)

      userEvent.type(screen.getByRole("textbox"), "doc")

      jest.advanceTimersByTime(FILTER_INPUT_DEBOUNCE / 2)

      // The debouncing time hasn't passed yet, so we don't expect to see any changes
      expect(getSearch()).toEqual("")

      userEvent.type(screen.getByRole("textbox"), "ker")

      // The debouncing time hasn't passed yet, so we don't expect to see any changes
      expect(getSearch()).toEqual("")

      jest.advanceTimersByTime(FILTER_INPUT_DEBOUNCE)

      // Since the debouncing time has passed, we expect to see the final
      // change reflected
      expect(getSearch()).toEqual("?term=docker")
    })

    it("retains any current level and source filters when its value changes", () => {
      jest.useFakeTimers()

      customRender(<FullBar />, {
        initialEntries: ["/?level=warn&source=build"],
      })

      userEvent.type(screen.getByRole("textbox"), "help")

      jest.advanceTimersByTime(FILTER_INPUT_DEBOUNCE)

      expect(getSearch()).toEqual("?level=warn&source=build&term=help")
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
      ).toBe("source=build&term=find+me")

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
      ).toBe("source=runtime")
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
      ).toBe("term=service")
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
