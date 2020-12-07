import { storiesOf } from "@storybook/react"
import React from "react"
import { MemoryRouter } from "react-router"
import PathBuilder from "./PathBuilder"
import SidebarItem from "./SidebarItem"
import { SidebarItemView } from "./SidebarResources"
import { oneResourceNoAlerts } from "./testdata"
import { ResourceStatus, ResourceView } from "./types"

type Resource = Proto.webviewResource
let pathBuilder = PathBuilder.forTesting("localhost", "/")

function oneItemWithStatus(status: ResourceStatus) {
  let item = new SidebarItem(oneResourceNoAlerts())
  item.status = status
  if (status === ResourceStatus.Building) {
    item.currentBuildStartTime = new Date(Date.now() - 1).toISOString()
  }
  if (
    status === ResourceStatus.Unhealthy ||
    status === ResourceStatus.Warning
  ) {
    item.alertCount = 1
  }
  return (
    <MemoryRouter initialEntries={["/"]}>
      <SidebarItemView
        item={item}
        selected={false}
        renderPin={true}
        resourceView={ResourceView.Log}
        pathBuilder={pathBuilder}
      />
    </MemoryRouter>
  )
}

storiesOf("SidebarItemView", module)
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
