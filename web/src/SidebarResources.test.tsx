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
  })
})
