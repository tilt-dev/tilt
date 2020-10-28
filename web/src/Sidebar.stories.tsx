import React from "react"
import { storiesOf } from "@storybook/react"
import SidebarResources from "./SidebarResources"
import SidebarItem from "./SidebarItem"
import {
  oneResourceView,
  oneResourceNoAlerts,
  twoResourceView,
} from "./testdata"
import PathBuilder from "./PathBuilder"
import { MemoryRouter } from "react-router"
import { ResourceStatus, ResourceView, TriggerMode } from "./types"
import Sidebar from "./Sidebar"

type Resource = Proto.webviewResource
let pathBuilder = PathBuilder.forTesting("localhost", "/")

function twoItemSidebar() {
  let items = twoResourceView().resources.map(
    (res: Resource) => new SidebarItem(res)
  )
  items[0].name = "snapshot-frontend-binary-long-name"
  return (
    <MemoryRouter initialEntries={["/"]}>
      <Sidebar isClosed={false} toggleSidebar={() => {}}>
        <SidebarResources
          items={items}
          selected=""
          resourceView={ResourceView.Log}
          pathBuilder={pathBuilder}
        />
      </Sidebar>
    </MemoryRouter>
  )
}

function twoItemSidebarClosed() {
  let items = twoResourceView().resources.map(
    (res: Resource) => new SidebarItem(res)
  )
  items[0].name = "snapshot-frontend-binary-long-name"
  return (
    <MemoryRouter initialEntries={["/"]}>
      <Sidebar isClosed={true} toggleSidebar={() => {}}>
        <SidebarResources
          items={items}
          selected=""
          resourceView={ResourceView.Log}
          pathBuilder={pathBuilder}
        />
      </Sidebar>
    </MemoryRouter>
  )
}

function twoItemSidebarOnePinned() {
  let items = twoResourceView().resources.map(
    (res: Resource) => new SidebarItem(res)
  )
  items[0].name = "snapshot-frontend-binary-long-name"
  return (
    <MemoryRouter initialEntries={["/"]}>
      <Sidebar isClosed={false} toggleSidebar={() => {}}>
        <SidebarResources
          items={items}
          selected=""
          resourceView={ResourceView.Log}
          pathBuilder={pathBuilder}
          initialPinnedItemsForTesting={[items[1].name]}
        />
      </Sidebar>
    </MemoryRouter>
  )
}

function oneItemWithTrigger() {
  let items = oneResourceView().resources.map((res: Resource) => {
    let item = new SidebarItem(res)
    item.triggerMode = TriggerMode.TriggerModeManualAfterInitial
    item.hasPendingChanges = true
    item.currentBuildStartTime = ""
    return item
  })
  return (
    <MemoryRouter initialEntries={["/"]}>
      <Sidebar isClosed={false} toggleSidebar={() => {}}>
        <SidebarResources
          items={items}
          selected=""
          resourceView={ResourceView.Log}
          pathBuilder={pathBuilder}
        />
      </Sidebar>
    </MemoryRouter>
  )
}

function oneItemWithStatus(status: ResourceStatus) {
  let item = new SidebarItem(oneResourceNoAlerts())
  item.status = status
  item.currentBuildStartTime = ""
  if (
    status === ResourceStatus.Unhealthy ||
    status === ResourceStatus.Warning
  ) {
    item.alertCount = 1
  }
  let items = [item]
  return (
    <MemoryRouter initialEntries={["/"]}>
      <Sidebar isClosed={false} toggleSidebar={() => {}}>
        <SidebarResources
          items={items}
          selected=""
          resourceView={ResourceView.Log}
          pathBuilder={pathBuilder}
        />
      </Sidebar>
    </MemoryRouter>
  )
}

storiesOf("Sidebar", module)
  .add("two-items", twoItemSidebar)
  .add("two-items-closed", twoItemSidebarClosed)
  .add("two-items-one-pinned", twoItemSidebarOnePinned)
  .add("one-item-with-trigger", oneItemWithTrigger)
  .add(
    "one-item-building",
    oneItemWithStatus.bind(null, ResourceStatus.Building)
  )
  .add("one-item-pending", oneItemWithStatus.bind(null, ResourceStatus.Pending))
  .add("one-item-healthy", oneItemWithStatus.bind(null, ResourceStatus.Healthy))
  .add(
    "one-item-unhealthy",
    oneItemWithStatus.bind(null, ResourceStatus.Unhealthy)
  )
  .add("one-item-warning", oneItemWithStatus.bind(null, ResourceStatus.Warning))
  .add("one-item-none", oneItemWithStatus.bind(null, ResourceStatus.None))
