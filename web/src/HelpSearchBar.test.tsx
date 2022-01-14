import { mount } from "enzyme"
import { MemoryRouter } from "react-router"
import { tiltfileKeyContext } from "./BrowserStorage"
import React from "react"

import {
  ClearHelpSearchBarButton,
  HelpSearchBar
} from "./HelpSearchBar"

const HelpSearchBarTestWrapper = (props: {defaultValue: string}) => (
  <MemoryRouter>
    <tiltfileKeyContext.Provider value="test">
      <HelpSearchBar defaultValue={props.defaultValue}/>
    </tiltfileKeyContext.Provider>
  </MemoryRouter>
)

describe("HelpSearchBar", () => {
  it("does NOT display 'clear' button when there is NO input", () => {
    const root = mount(<HelpSearchBarTestWrapper defaultValue=""/>)
    const button = root.find(ClearHelpSearchBarButton)
    expect(button.length).toBe(0)
  })

  it("displays 'clear' button when there is input", () => {
    const root = mount(<HelpSearchBarTestWrapper defaultValue="much string" />)
    const button = root.find(ClearHelpSearchBarButton)
    expect(button.length).toBe(1)
  })
})