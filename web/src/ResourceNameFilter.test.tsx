import { render, RenderOptions, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import React from "react"
import { MemoryRouter } from "react-router"
import { accessorsForTesting, tiltfileKeyContext } from "./BrowserStorage"
import {
  DEFAULT_OPTIONS,
  ResourceListOptions,
  ResourceListOptionsProvider,
  RESOURCE_LIST_OPTIONS_KEY,
} from "./ResourceListOptionsContext"
import { ResourceNameFilter } from "./ResourceNameFilter"

const resourceListOptionsAccessor = accessorsForTesting<ResourceListOptions>(
  RESOURCE_LIST_OPTIONS_KEY,
  sessionStorage
)

function customRender(component: JSX.Element, options?: RenderOptions) {
  return render(component, {
    wrapper: ({ children }) => (
      <MemoryRouter
        future={{ v7_startTransition: true, v7_relativeSplatPath: true }}
      >
        <tiltfileKeyContext.Provider value="test">
          <ResourceListOptionsProvider>{children}</ResourceListOptionsProvider>
        </tiltfileKeyContext.Provider>
      </MemoryRouter>
    ),
  })
}

describe("ResourceNameFilter", () => {
  beforeEach(() => {
    sessionStorage.clear()
    localStorage.clear()
  })

  afterEach(() => {
    sessionStorage.clear()
    localStorage.clear()
  })

  it("displays 'clear' button when there is input", () => {
    resourceListOptionsAccessor.set({
      ...DEFAULT_OPTIONS,
      resourceNameFilter: "wow",
    })
    customRender(<ResourceNameFilter />)

    expect(screen.getByLabelText("Clear name filter")).toBeInTheDocument()
  })

  it("does NOT display 'clear' button when there is NO input", () => {
    customRender(<ResourceNameFilter />)

    expect(screen.queryByLabelText("Clear name filter")).toBeNull()
  })

  describe("persistent state", () => {
    it("reflects existing value in ResourceListOptionsContext", () => {
      resourceListOptionsAccessor.set({
        ...DEFAULT_OPTIONS,
        resourceNameFilter: "cool resource",
      })
      customRender(<ResourceNameFilter />)

      expect(screen.getByRole("textbox")).toHaveValue("cool resource")
    })

    it("saves input to ResourceListOptionsContext", () => {
      customRender(<ResourceNameFilter />)

      userEvent.type(screen.getByRole("textbox"), "very cool resource")

      expect(resourceListOptionsAccessor.get()?.resourceNameFilter).toBe(
        "very cool resource"
      )
    })
  })
})
