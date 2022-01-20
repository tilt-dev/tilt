import { mount } from "enzyme"
import { MemoryRouter } from "react-router"
import { tiltfileKeyContext } from "./BrowserStorage"
import { ClearHelpSearchBarButton, HelpSearchBar } from "./HelpSearchBar"

const HelpSearchBarTestWrapper = () => (
  <MemoryRouter>
    <tiltfileKeyContext.Provider value="test">
      <HelpSearchBar />
    </tiltfileKeyContext.Provider>
  </MemoryRouter>
)

describe("HelpSearchBar", () => {
  it("does NOT display 'clear' button when there is NO input", () => {
    const root = mount(<HelpSearchBarTestWrapper />)
    const button = root.find(ClearHelpSearchBarButton)
    expect(button.length).toBe(0)
  })

  it("displays 'clear' button when there is input", () => {
    const searchTerm = "wow"
    const root = mount(<HelpSearchBarTestWrapper />)
    const searchField = root.find("input")
    searchField.simulate("change", { target: { value: searchTerm } })

    const button = root.find(ClearHelpSearchBarButton)
    expect(button.length).toBe(1)
  })

  it("should change the search value on input change", () => {
    const searchTerm = "so search"
    const root = mount(<HelpSearchBarTestWrapper />)
    const searchField = root.find("input")
    searchField.simulate("change", { target: { value: searchTerm } })

    const searchFieldAfterChange = root.find("input")
    expect(searchFieldAfterChange.prop("value")).toBe(searchTerm)
  })

  it("should open search in new tab on submision", () => {
    const windowOpenSpy = jest.fn()
    window.open = windowOpenSpy
    const searchTerm = "such term"
    const searchResultsPage = new URL(`https://docs.tilt.dev/search`)
    searchResultsPage.searchParams.set("q", searchTerm)
    searchResultsPage.searchParams.set("utm_source", "tiltui")

    const root = mount(<HelpSearchBarTestWrapper />)
    const searchField = root.find("input")
    searchField.simulate("change", { target: { value: searchTerm } })
    searchField.simulate("keyPress", { key: "Enter" })

    expect(windowOpenSpy).toBeCalledWith(searchResultsPage)
  })

  it("should clear the search value after submission", () => {
    const searchTerm = "much find"
    const root = mount(<HelpSearchBarTestWrapper />)
    const searchField = root.find("input")
    searchField.simulate("change", { target: { value: searchTerm } })
    searchField.simulate("keyPress", { key: "Enter" })

    const searchFieldAfterChange = root.find("input")
    expect(searchFieldAfterChange.prop("value")).toBe("")
  })
})
