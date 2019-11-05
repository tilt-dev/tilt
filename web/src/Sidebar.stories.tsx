import React from "react"
import { storiesOf } from "@storybook/react"
import Sidebar, { SidebarItem } from "./Sidebar"
import { oneResourceView, twoResourceView } from "./testdata"
import PathBuilder from "./PathBuilder"
import { MemoryRouter } from "react-router"
import { ResourceView, TriggerMode, Resource } from "./types"

let pathBuilder = new PathBuilder("localhost", "/")

function twoItemSidebar() {
  let items = twoResourceView().resources.map(
    (res: Resource) => new SidebarItem(res)
  )
  return (
    <MemoryRouter initialEntries={["/"]}>
      <Sidebar
        isClosed={false}
        items={items}
        selected=""
        toggleSidebar={null}
        resourceView={ResourceView.Log}
        pathBuilder={pathBuilder}
      />
    </MemoryRouter>
  )
}
function oneItemWithTrigger() {
  let items = oneResourceView().resources.map((res: Resource) => {
    let item = new SidebarItem(res)
    item.triggerMode = TriggerMode.TriggerModeManual
    item.hasPendingChanges = true
    item.currentBuildStartTime = ""
    return item
  })
  return (
    <MemoryRouter initialEntries={["/"]}>
      <Sidebar
        isClosed={false}
        items={items}
        selected=""
        toggleSidebar={null}
        resourceView={ResourceView.Log}
        pathBuilder={pathBuilder}
      />
    </MemoryRouter>
  )
}

storiesOf("Sidebar", module)
  .add("two-items", twoItemSidebar)
  .add("one-item-with-trigger", oneItemWithTrigger)
