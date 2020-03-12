import React from "react"
import { storiesOf } from "@storybook/react"
import Sidebar, { SidebarItem } from "./Sidebar"
import {
  oneResourceView,
  oneResourceNoAlerts,
  twoResourceView,
} from "./testdata"
import PathBuilder from "./PathBuilder"
import { MemoryRouter } from "react-router"
import { ResourceStatus, ResourceView, TriggerMode } from "./types"

type Resource = Proto.webviewResource
let pathBuilder = new PathBuilder("localhost", "/")

function twoItemSidebar() {
  let items = twoResourceView().resources.map(
    (res: Resource) => new SidebarItem(res)
  )
  items[0].name = "snapshot-frontend-binary-long-name"
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

function twoItemSidebarClosed() {
  let items = twoResourceView().resources.map(
    (res: Resource) => new SidebarItem(res)
  )
  items[0].name = "snapshot-frontend-binary-long-name"
  return (
    <MemoryRouter initialEntries={["/"]}>
      <Sidebar
        isClosed={true}
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
    item.triggerMode = TriggerMode.TriggerModeManualAfterInitial
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
  .add("two-items-closed", twoItemSidebarClosed)
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
