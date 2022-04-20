import { render, RenderOptions, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { createMemoryHistory, MemoryHistory } from "history"
import React, { ChangeEvent, useState } from "react"
import { act } from "react-dom/test-utils"
import { Router } from "react-router"
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

// history
function customRender(
  wrapperOptions: {
    history: MemoryHistory
    validateOverride?: (name: string) => boolean
  },
  options?: RenderOptions
) {
  const validateResource =
    wrapperOptions.validateOverride ??
    function (name: string) {
      return name !== INVALID_RESOURCE
    }
  return render(<TestResourceNavConsumer />, {
    wrapper: ({ children }) => (
      <Router history={wrapperOptions.history}>
        <ResourceNavProvider validateResource={validateResource}>
          {children}
        </ResourceNavProvider>
      </Router>
    ),
    ...options,
  })
}

describe("ResourceNavContext", () => {
  it("navigates to resource on click", () => {
    const history = createMemoryHistory()
    customRender({ history })

    expect(screen.getByLabelText("selectedResource")).toHaveTextContent("")

    userEvent.type(screen.getByRole("textbox"), "res1")
    userEvent.click(screen.getByRole("button", { name: "openResource" }))

    expect(screen.getByLabelText("selectedResource")).toHaveTextContent("res1")
    expect(history.location.pathname).toEqual("/r/res1/overview")
  })

  it("filters resources that don't validate", () => {
    const history = createMemoryHistory()
    // Set location to invalid resource
    history.location.pathname = `/r/${INVALID_RESOURCE}/overview`
    customRender({ history })

    expect(screen.getByLabelText("selectedResource")).toHaveTextContent("")
    expect(screen.getByLabelText("invalidResource")).toHaveTextContent(
      INVALID_RESOURCE
    )
  })

  it("always validates the 'all' resource", () => {
    const history = createMemoryHistory()
    // Set location to 'all' resource
    history.location.pathname = `/r/${ResourceName.all}/overview`
    customRender({ history, validateOverride: (_name: string) => false })

    expect(screen.getByLabelText("selectedResource")).toHaveTextContent(
      ResourceName.all
    )
    expect(screen.getByLabelText("invalidResource")).toHaveTextContent("")
  })

  it("encodes resource names", () => {
    const history = createMemoryHistory()
    customRender({ history })

    userEvent.type(screen.getByRole("textbox"), "foo/bar")
    userEvent.click(screen.getByRole("button", { name: "openResource" }))

    expect(screen.getByLabelText("selectedResource")).toHaveTextContent(
      "foo/bar"
    )
    expect(history.location.pathname).toEqual("/r/foo%2Fbar/overview")
  })

  // Make sure that useResourceNav() doesn't break memoization.
  it("memoizes renders", () => {
    let renderCount = 0
    let FakeEl = React.memo(() => {
      useResourceNav()
      renderCount++
      return <div></div>
    })

    let history = createMemoryHistory()
    let validateResource = () => true
    let { rerender } = render(
      <Router history={history}>
        <ResourceNavProvider validateResource={validateResource}>
          <FakeEl />
        </ResourceNavProvider>
      </Router>
    )

    expect(renderCount).toEqual(1)

    // Make sure we don't re-render on a no-op history update.
    rerender(
      <Router history={history}>
        <ResourceNavProvider validateResource={validateResource}>
          <FakeEl />
        </ResourceNavProvider>
      </Router>
    )
    expect(renderCount).toEqual(1)

    // Make sure we do re-render on a real location update.
    act(() => history.push("/r/foo"))
    expect(renderCount).toEqual(2)
  })
})
