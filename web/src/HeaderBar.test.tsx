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
import { ThemeProvider } from "./ThemeContext"

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

    it("opens the keyboard shortcuts dialog on '?' keypress", () => {
      // Expect that the dialog is NOT visible at start
      expect(
        screen.queryByRole("heading", { name: /Keyboard Shortcuts/i })
      ).toBeNull()

      act(() => {
        userEvent.keyboard("?")
      })

      expect(
        screen.getByRole("heading", { name: /Keyboard Shortcuts/i })
      ).toBeInTheDocument()
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

  describe("settings menu", () => {
    const openModal = jest.fn()

    beforeEach(() => {
      localStorage.clear()
      openModal.mockReset()
      render(
        <MemoryRouter
          initialEntries={["/"]}
          future={{ v7_startTransition: true, v7_relativeSplatPath: true }}
        >
          <ThemeProvider>
            <tiltfileKeyContext.Provider value="test">
              <ResourceListOptionsProvider>
                <SnapshotActionTestProvider
                  value={{ enabled: true, openModal }}
                >
                  <HeaderBar
                    view={nResourceView(2)}
                    currentPage={HeaderBarPage.Detail}
                    isSocketConnected={true}
                  />
                </SnapshotActionTestProvider>
              </ResourceListOptionsProvider>
            </tiltfileKeyContext.Provider>
          </ThemeProvider>
        </MemoryRouter>
      )
      userEvent.click(screen.getByRole("menuitem", { name: "Settings" }))
    })

    it("opens the snapshot modal", () => {
      userEvent.click(screen.getByRole("menuitem", { name: /snapshot/i }))
      expect(openModal).toBeCalledTimes(1)
    })

    it("toggles the theme", () => {
      userEvent.click(
        screen.getByRole("menuitem", { name: /Switch to light theme/i })
      )
      expect(document.body.getAttribute("data-theme")).toBe("light")
      userEvent.click(
        screen.getByRole("menuitem", { name: /Switch to dark theme/i })
      )
      expect(document.body.getAttribute("data-theme")).toBeNull()
    })

    it("opens the keyboard shortcuts dialog", () => {
      userEvent.click(
        screen.getByRole("menuitem", { name: /Keyboard shortcuts/i })
      )
      expect(
        screen.getByRole("heading", { name: /Keyboard Shortcuts/i })
      ).toBeInTheDocument()
      expect(screen.queryByRole("menuitem", { name: /Switch to/i })).toBeNull()
    })
  })
})
