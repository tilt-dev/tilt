import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { act } from "react"
import { MemoryRouter } from "react-router-dom"
import { tiltfileKeyContext } from "./BrowserStorage"
import HeaderBar, { HeaderBarPage } from "./HeaderBar"
import { ResourceListOptionsProvider } from "./ResourceListOptionsContext"
import { ResourceNameFilter } from "./ResourceNameFilter"
import { SnapshotActionTestProvider } from "./snapshot"
import { nResourceView } from "./testdata"

describe("HeaderBar", () => {
  describe("keyboard shortcuts", () => {
    const openModal = jest.fn()

    beforeEach(() => {
      openModal.mockReset()

      const snapshotAction = {
        enabled: true,
        openModal,
      }

      render(
        <MemoryRouter
          initialEntries={["/"]}
          future={{ v7_startTransition: true, v7_relativeSplatPath: true }}
        >
          <tiltfileKeyContext.Provider value="test">
            <ResourceListOptionsProvider>
              <SnapshotActionTestProvider value={snapshotAction}>
                <HeaderBar
                  view={nResourceView(2)}
                  currentPage={HeaderBarPage.Detail}
                  isSocketConnected={true}
                />
                <ResourceNameFilter />
              </SnapshotActionTestProvider>
            </ResourceListOptionsProvider>
          </tiltfileKeyContext.Provider>
        </MemoryRouter>
      )
    })

    it("opens the help dialog on '?' keypress", () => {
      // Expect that the help dialog is NOT visible at start
      expect(screen.queryByRole("heading", { name: /Help/i })).toBeNull()

      act(() => {
        userEvent.keyboard("?")
      })

      expect(screen.getByRole("heading", { name: /Help/i })).toBeInTheDocument()
    })

    it("calls `openModal` snapshot callback on 's' keypress", () => {
      expect(openModal).not.toBeCalled()

      act(() => {
        userEvent.keyboard("s")
      })

      expect(openModal).toBeCalledTimes(1)
    })

    it("focuses and selects the resource name filter on '/' keypress", () => {
      const filter = screen.getByPlaceholderText(
        "Filter resources by name"
      ) as HTMLInputElement
      userEvent.type(filter, "existing")
      filter.blur()
      expect(filter).not.toHaveFocus()

      act(() => {
        userEvent.keyboard("/")
      })

      expect(filter).toHaveFocus()
      expect(filter.selectionStart).toBe(0)
      expect(filter.selectionEnd).toBe(filter.value.length)
    })
  })
})
