import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import React from "react"
import { act } from "react-dom/test-utils"
import { MemoryRouter } from "react-router-dom"
import HeaderBar, { HeaderBarPage } from "./HeaderBar"
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
        <MemoryRouter initialEntries={["/"]}>
          <SnapshotActionTestProvider value={snapshotAction}>
            <HeaderBar
              view={nResourceView(2)}
              currentPage={HeaderBarPage.Detail}
              isSocketConnected={true}
            />
          </SnapshotActionTestProvider>
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
  })
})
