import { render, RenderOptions, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { MemoryRouter } from "react-router"
import { tiltfileKeyContext } from "./BrowserStorage"
import { HelpSearchBar } from "./HelpSearchBar"

function customRender(component: JSX.Element, options?: RenderOptions) {
  return render(
    <MemoryRouter>
      <tiltfileKeyContext.Provider value="test">
        {component}
      </tiltfileKeyContext.Provider>
    </MemoryRouter>,
    options
  )
}

describe("HelpSearchBar", () => {
  it("does NOT display 'clear' button when there is NO input", () => {
    customRender(<HelpSearchBar />)

    expect(screen.queryByLabelText("Clear search term")).toBeNull()
  })

  it("displays 'clear' button when there is input", () => {
    customRender(<HelpSearchBar />)

    userEvent.type(screen.getByLabelText("Search Tilt Docs"), "wow")

    expect(screen.getByLabelText("Clear search term")).toBeInTheDocument()
  })

  it("should change the search value on input change", () => {
    const searchTerm = "so search"
    customRender(<HelpSearchBar />)

    userEvent.type(screen.getByLabelText("Search Tilt Docs"), searchTerm)

    expect(screen.getByRole("textbox")).toHaveValue(searchTerm)
  })

  it("should open search in new tab on submision", () => {
    const windowOpenSpy = jest.fn()
    window.open = windowOpenSpy
    const searchTerm = "such term"
    const searchResultsPage = new URL(`https://docs.tilt.dev/search`)
    searchResultsPage.searchParams.set("q", searchTerm)
    searchResultsPage.searchParams.set("utm_source", "tiltui")

    customRender(<HelpSearchBar />)

    userEvent.type(screen.getByLabelText("Search Tilt Docs"), searchTerm)
    userEvent.keyboard("{Enter}")

    expect(windowOpenSpy).toBeCalledWith(searchResultsPage)
  })

  it("should clear the search value after submission", () => {
    const searchTerm = "much find"
    customRender(<HelpSearchBar />)

    userEvent.type(screen.getByLabelText("Search Tilt Docs"), searchTerm)
    userEvent.keyboard("{Enter}")

    expect(screen.getByRole("textbox")).toHaveValue("")
  })
})
