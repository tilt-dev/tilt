import React from "react"
import { mount, ReactWrapper } from "enzyme"
import { ResourceView } from "./types"
import { twoResourceView } from "./testdata"
import SidebarResources, {
  SidebarItemLink,
  SidebarListSection,
} from "./SidebarResources"
import SidebarItem from "./SidebarItem"
import { MemoryRouter } from "react-router"
import PathBuilder from "./PathBuilder"
import { SidebarPinButton } from "./SidebarPin"
import { expectIncr } from "./analytics_test_helpers"
import fetchMock from "jest-fetch-mock"
import { LocalStorageContextProvider, makeKey } from "./LocalStorage"

let pathBuilder = PathBuilder.forTesting("localhost", "/")

function getPinnedItemNames(
  root: ReactWrapper<any, React.Component["state"], React.Component>
): Array<string> {
  let pinnedItems = root
    .find(SidebarListSection)
    .find({ name: "favorites" })
    .find(SidebarItemLink)
  return pinnedItems.map((i) => i.prop("title"))
}

function clickPin(
  root: ReactWrapper<any, React.Component["state"], React.Component>,
  name: string
) {
  let pinButton = root.find(SidebarPinButton).find({ resourceName: name })
  expect(pinButton).toHaveLength(1)
  pinButton.simulate("click")
}

describe("SidebarResources", () => {
  beforeEach(() => {
    fetchMock.resetMocks()
    fetchMock.mockResponse(JSON.stringify({}))
  })

  afterEach(() => {
    fetchMock.resetMocks()
    localStorage.clear()
  })

  it("adds items to the pinned group when items are pinned", () => {
    let items = twoResourceView().resources.map((r) => new SidebarItem(r))
    const root = mount(
      <MemoryRouter>
        <LocalStorageContextProvider tiltfileKey={"test"}>
          <SidebarResources
            items={items}
            selected={""}
            resourceView={ResourceView.Log}
            pathBuilder={pathBuilder}
          />
        </LocalStorageContextProvider>
      </MemoryRouter>
    )

    expect(getPinnedItemNames(root)).toEqual([])

    clickPin(root, "snack")

    expect(getPinnedItemNames(root)).toEqual(["snack"])

    expectIncr(0, "ui.web.pin", { pinCount: "0", action: "load" })
    expectIncr(1, "ui.web.pin", { pinCount: "1", action: "pin" })

    expect(localStorage.getItem(makeKey("test", "pinned-resources"))).toEqual(
      JSON.stringify(["snack"])
    )
  })

  it("reads pinned items from local storage", () => {
    localStorage.setItem(
      makeKey("test", "pinned-resources"),
      JSON.stringify(["vigoda", "snack"])
    )

    let items = twoResourceView().resources.map((r) => new SidebarItem(r))
    const root = mount(
      <MemoryRouter>
        <LocalStorageContextProvider tiltfileKey={"test"}>
          <SidebarResources
            items={items}
            selected={""}
            resourceView={ResourceView.Log}
            pathBuilder={pathBuilder}
          />
        </LocalStorageContextProvider>
      </MemoryRouter>
    )

    expect(getPinnedItemNames(root)).toEqual(["vigoda", "snack"])
  })

  it("removes items from the pinned group when items are pinned", () => {
    let items = twoResourceView().resources.map((r) => new SidebarItem(r))
    localStorage.setItem(
      makeKey("test", "pinned-resources"),
      JSON.stringify(items.map((i) => i.name))
    )

    const root = mount(
      <MemoryRouter>
        <LocalStorageContextProvider tiltfileKey={"test"}>
          <SidebarResources
            items={items}
            selected={""}
            resourceView={ResourceView.Log}
            pathBuilder={pathBuilder}
          />
        </LocalStorageContextProvider>
      </MemoryRouter>
    )

    expect(getPinnedItemNames(root)).toEqual(["vigoda", "snack"])

    clickPin(root, "snack")

    expect(getPinnedItemNames(root)).toEqual(["vigoda"])

    expectIncr(0, "ui.web.pin", { pinCount: "2", action: "load" })
    expectIncr(1, "ui.web.pin", { pinCount: "1", action: "unpin" })

    expect(localStorage.getItem(makeKey("test", "pinned-resources"))).toEqual(
      JSON.stringify(["vigoda"])
    )
  })
})
