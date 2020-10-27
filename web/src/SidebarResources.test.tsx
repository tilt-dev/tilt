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
import {expectIncr} from "./analytics_test_helpers"
import fetchMock from "jest-fetch-mock"

let pathBuilder = PathBuilder.forTesting("localhost", "/")

function getPinnedItemNames(
  root: ReactWrapper<any, React.Component["state"], React.Component>
): Array<string> {
  let pinnedItems = root
    .find(SidebarListSection)
    .find({ name: "favorites" })
    .find(SidebarItemLink)
  return pinnedItems.map(i => i.prop("title"))
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
  })

  it("adds items to the pinned group when items are pinned", () => {
    let items = twoResourceView().resources.map(r => new SidebarItem(r))
    const root = mount(
      <MemoryRouter>
        <SidebarResources
          items={items}
          selected={""}
          resourceView={ResourceView.Log}
          pathBuilder={pathBuilder}
        />
      </MemoryRouter>
    )

    expect(getPinnedItemNames(root)).toEqual([])

    clickPin(root, "snack")

    expect(getPinnedItemNames(root)).toEqual(["snack"])

    expectIncr(0, 'ui.web.pin', {newPinCount: '1', pinning: 'true'})
  })

  it("adds items to the pinned group when items are pinned", () => {
    let items = twoResourceView().resources.map(r => new SidebarItem(r))
    const root = mount(
      <MemoryRouter>
        <SidebarResources
          items={items}
          selected={""}
          resourceView={ResourceView.Log}
          pathBuilder={pathBuilder}
          initialPinnedItems={["vigoda", "snack"]}
        />
      </MemoryRouter>
    )

    expect(getPinnedItemNames(root)).toEqual(["vigoda", "snack"])

    clickPin(root, "snack")

    expect(getPinnedItemNames(root)).toEqual(["vigoda"])

    expectIncr(0, 'ui.web.pin', {newPinCount: '1', pinning: 'false'})
  })
})
