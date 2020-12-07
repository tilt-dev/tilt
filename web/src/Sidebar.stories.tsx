import React from "react"
import { MemoryRouter } from "react-router"
import PathBuilder from "./PathBuilder"
import Sidebar from "./Sidebar"
import SidebarItem from "./SidebarItem"
import SidebarResources from "./SidebarResources"
import {
  oneResourceNoAlerts,
  oneResourceView,
  twoResourceView,
} from "./testdata"
import { ResourceStatus, ResourceView, TriggerMode } from "./types"

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

export default {
  title: "Sidebar",
}

export const TwoItems = twoItemSidebar

export const TwoItemsClosed = twoItemSidebarClosed

export const TwoItemsOnePinned = twoItemSidebarOnePinned

export const OneItemWithTrigger = oneItemWithTrigger

export const OneItemBuilding = oneItemWithStatus.bind(
  null,
  ResourceStatus.Building
)

export const OneItemPending = oneItemWithStatus.bind(
  null,
  ResourceStatus.Pending
)

export const OneItemHealthy = oneItemWithStatus.bind(
  null,
  ResourceStatus.Healthy
)

export const OneItemUnhealthy = oneItemWithStatus.bind(
  null,
  ResourceStatus.Unhealthy
)

export const OneItemWarning = oneItemWithStatus.bind(
  null,
  ResourceStatus.Warning
)

export const OneItemNone = oneItemWithStatus.bind(null, ResourceStatus.None)
