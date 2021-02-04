import { mount, ReactWrapper } from "enzyme"
import React from "react"
import { MemoryRouter } from "react-router"
import { makeKey, tiltfileKeyContext } from "./LocalStorage"
import { TwoResourcesTwoTests } from "./OverviewResourceSidebar.stories"
import { OverviewSidebarOptions } from "./OverviewSidebarOptions"
import PathBuilder from "./PathBuilder"
import SidebarItem from "./SidebarItem"
import SidebarItemView from "./SidebarItemView"
import { SidebarPinContextProvider } from "./SidebarPin"
import SidebarResources, { SidebarListSection } from "./SidebarResources"
import { oneResource, tiltfileResource } from "./testdata"
import { ResourceView } from "./types"

let pathBuilder = PathBuilder.forTesting("localhost", "/")

function assertSidebarItemsAndOptions(
  root: ReactWrapper,
  names: string[],
  expectShowResources: boolean,
  expectShowTests: boolean
) {
  let sidebar = root.find(SidebarResources)
  expect(sidebar).toHaveLength(1)

  // only check items in the "all resources" section, i.e. don't look at pinned things
  // or we'll have duplicates
  let all = sidebar.find(SidebarListSection).find({ name: "resources" })
  let items = all.find(SidebarItemView)
  expect(items).toHaveLength(names.length)

  for (let i = 0; i < names.length; i++) {
    expect(items.at(i).props().item.name).toEqual(names[i])
  }

  let optSetter = sidebar.find(OverviewSidebarOptions)
  expect(optSetter).toHaveLength(1)
  expect(optSetter.find("input#resources").props().checked).toEqual(
    expectShowResources
  )
  expect(optSetter.find("input#tests").props().checked).toEqual(expectShowTests)
}

const allNames = ["(Tiltfile)", "vigoda", "snack", "beep", "boop"]

describe("overview sidebar options", () => {
  afterEach(() => {
    localStorage.clear()
  })

  it("shows tests and resources by default", () => {
    const root = mount(TwoResourcesTwoTests())
    assertSidebarItemsAndOptions(root, allNames, true, true)
  })

  it("hides resources when resources unchecked", () => {
    const root = mount(TwoResourcesTwoTests())
    assertSidebarItemsAndOptions(root, allNames, true, true)

    root
      .find("input#resources")
      .simulate("change", { target: { checked: false } })
    assertSidebarItemsAndOptions(
      root,
      ["(Tiltfile)", "beep", "boop"],
      false,
      true
    )
  })

  it("hides tests when tests unchecked", () => {
    const root = mount(TwoResourcesTwoTests())
    assertSidebarItemsAndOptions(root, allNames, true, true)

    root.find("input#tests").simulate("change", { target: { checked: false } })
    assertSidebarItemsAndOptions(
      root,
      ["(Tiltfile)", "vigoda", "snack"],
      true,
      false
    )
  })

  it("hides resources and tests when both unchecked", () => {
    const root = mount(TwoResourcesTwoTests())
    assertSidebarItemsAndOptions(root, allNames, true, true)

    root
      .find("input#resources")
      .simulate("change", { target: { checked: false } })
    root.find("input#tests").simulate("change", { target: { checked: false } })
    assertSidebarItemsAndOptions(root, ["(Tiltfile)"], false, false)
  })

  it("doesn't show SidebarOptionSetter if no tests present", () => {
    let items = [tiltfileResource(), oneResource()].map(
      (r) => new SidebarItem(r)
    )
    const root = mount(
      <MemoryRouter>
        <tiltfileKeyContext.Provider value="test">
          <SidebarPinContextProvider>
            <SidebarResources
              items={items}
              selected={""}
              resourceView={ResourceView.Log}
              pathBuilder={pathBuilder}
            />
          </SidebarPinContextProvider>
        </tiltfileKeyContext.Provider>
      </MemoryRouter>
    )
    let sidebar = root.find(SidebarResources)
    expect(sidebar).toHaveLength(1)

    let optSetter = sidebar.find(OverviewSidebarOptions)
    expect(optSetter).toHaveLength(0)
  })

  it("still displays pinned tests when tests hidden", () => {
    localStorage.setItem(
      makeKey("test", "pinned-resources"),
      JSON.stringify(["beep"])
    )
    const root = mount(
      <MemoryRouter>
        <tiltfileKeyContext.Provider value="test">
          <SidebarPinContextProvider>
            {TwoResourcesTwoTests()}
          </SidebarPinContextProvider>
        </tiltfileKeyContext.Provider>
      </MemoryRouter>
    )

    assertSidebarItemsAndOptions(
      root,
      ["(Tiltfile)", "vigoda", "snack", "beep", "boop"],
      true,
      true
    )

    let pinned = root
      .find(SidebarListSection)
      .find({ name: "Pinned" })
      .find(SidebarItemView)
    expect(pinned).toHaveLength(1)
    expect(pinned.at(0).props().item.name).toEqual("beep")

    root.find("input#tests").simulate("change", { target: { checked: false } })
    assertSidebarItemsAndOptions(
      root,
      ["(Tiltfile)", "vigoda", "snack"],
      true,
      false
    )

    // "beep" should still be pinned, even though we're no longer showing tests in the main resource list
    pinned = root
      .find(SidebarListSection)
      .find({ name: "Pinned" })
      .find(SidebarItemView)
    expect(pinned).toHaveLength(1)
    expect(pinned.at(0).props().item.name).toEqual("beep")
  })
})

// TODO:
//   - if test present; hide/show tests/resources; and then test removed (e.g. commented
//     out of tiltfile) then we hide the check boxes AND ALSO reset filters to show everything
