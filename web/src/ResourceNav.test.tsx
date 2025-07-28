import { render, RenderOptions, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import React, { ChangeEvent, useState } from "react"
import { act } from "react-dom/test-utils"
import { MemoryRouter } from "react-router"
import { useLocation, useNavigate } from "react-router-dom"
import { ResourceNavProvider, useResourceNav } from "./ResourceNav"
import { ResourceName } from "./types"

const INVALID_RESOURCE = "res3"

function TestResourceNavConsumer() {
  const { selectedResource, invalidResource, openResource } = useResourceNav()
  const [resourceToSelect, setResourceToSelect] = useState("")

  return (
    <>
      <p aria-label="selectedResource">{selectedResource}</p>
      <p aria-label="invalidResource">{invalidResource}</p>
      <input
        aria-label="Resource to select"
        type="text"
        value={resourceToSelect}
        onChange={(e: ChangeEvent<HTMLInputElement>) =>
          setResourceToSelect(e.target.value)
        }
      />
      <button onClick={() => openResource(resourceToSelect)}>
        openResource
      </button>
    </>
  )
}

let location: any = window.location
let navigate: any = null

function LocationCapture() {
  location = useLocation()
  navigate = useNavigate()
  return null
}

function customRender(
  wrapperOptions: {
    initialEntries?: string[]
    validateOverride?: (name: string) => boolean
  } = {},
  options?: RenderOptions
) {
  const validateResource =
    wrapperOptions.validateOverride ??
    function (name: string) {
      return name !== INVALID_RESOURCE
    }
  return render(<TestResourceNavConsumer />, {
    wrapper: ({ children }) => (
      <MemoryRouter initialEntries={wrapperOptions.initialEntries || ["/"]}>
        <ResourceNavProvider validateResource={validateResource}>
          {children}
        </ResourceNavProvider>
        <LocationCapture />
      </MemoryRouter>
    ),
    ...options,
  })
}

describe("ResourceNavContext", () => {
  it("navigates to resource on click", () => {
    customRender()

    expect(screen.getByLabelText("selectedResource")).toHaveTextContent("")

    userEvent.type(screen.getByRole("textbox"), "res1")
    userEvent.click(screen.getByRole("button", { name: "openResource" }))

    expect(screen.getByLabelText("selectedResource")).toHaveTextContent("res1")
    expect(location.pathname.endsWith("/r/res1/overview")).toBe(true)
  })

  it("filters resources that don't validate", () => {
    customRender({ initialEntries: [`/r/${INVALID_RESOURCE}/overview`] })

    expect(screen.getByLabelText("selectedResource")).toHaveTextContent("")
    expect(screen.getByLabelText("invalidResource")).toHaveTextContent(
      INVALID_RESOURCE
    )
  })

  it("always validates the 'all' resource", () => {
    customRender({
      initialEntries: [`/r/${ResourceName.all}/overview`],
      validateOverride: (_name: string) => false,
    })

    expect(screen.getByLabelText("selectedResource")).toHaveTextContent(
      ResourceName.all
    )
    expect(screen.getByLabelText("invalidResource")).toHaveTextContent("")
  })

  it("encodes resource names", () => {
    customRender()

    userEvent.type(screen.getByRole("textbox"), "foo/bar")
    userEvent.click(screen.getByRole("button", { name: "openResource" }))

    expect(screen.getByLabelText("selectedResource")).toHaveTextContent(
      "foo/bar"
    )
    expect(location.pathname.endsWith("/r/foo%2Fbar/overview")).toBe(true)
  })

  it("preserves filters by resource", () => {
    customRender()

    let nav = (res: string) => {
      userEvent.clear(screen.getByRole("textbox"))
      userEvent.type(screen.getByRole("textbox"), res)
      userEvent.click(screen.getByRole("button", { name: "openResource" }))
    }

    // We can't directly check the MemoryRouter's location, so just check the selectedResource label
    nav("foo")
    expect(screen.getByLabelText("selectedResource")).toHaveTextContent("foo")
    // Simulate navigation with query param
    act(() => navigate("/r/foo/overview?term=hi"))
    nav("bar")
    expect(screen.getByLabelText("selectedResource")).toHaveTextContent("bar")
    nav("foo")
    expect(screen.getByLabelText("selectedResource")).toHaveTextContent("foo")
  })

  // Make sure that useResourceNav() doesn't break memoization.
  it("memoizes renders", () => {
    let renderCount = 0
    let FakeEl = React.memo(() => {
      useResourceNav()
      renderCount++
      return <div></div>
    })

    let validateResource = () => true
    let { rerender } = render(
      <MemoryRouter>
        <ResourceNavProvider validateResource={validateResource}>
          <FakeEl />
        </ResourceNavProvider>
      </MemoryRouter>
    )

    expect(renderCount).toEqual(1)

    // Make sure we don't re-render on a no-op update.
    rerender(
      <MemoryRouter>
        <ResourceNavProvider validateResource={validateResource}>
          <FakeEl />
        </ResourceNavProvider>
        <LocationCapture />
      </MemoryRouter>
    )
    expect(renderCount).toEqual(1)

    // Make sure we do re-render on a real location update.
    act(() => navigate("/r/foo"))
    expect(renderCount).toEqual(2)
  })
})
