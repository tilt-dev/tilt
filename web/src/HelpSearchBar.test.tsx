import { mount } from "enzyme"
import { MemoryRouter } from "react-router"
import { tiltfileKeyContext } from "./BrowserStorage"

import {
  ClearHelpSearchBarButton,
  searchValue,
  HelpSearchBar
} from "./HelpSearchBar"

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

})