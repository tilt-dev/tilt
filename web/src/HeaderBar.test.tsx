import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import React from "react"
import { act } from "react-dom/test-utils"
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

    it("focuses the resource name filter on '/' keypress", () => {
      const filter = screen.getByPlaceholderText("Filter resources by name")
      expect(filter).not.toHaveFocus()

      act(() => {
        userEvent.keyboard("/")
      })

      expect(filter).toHaveFocus()
    })
  })
})
