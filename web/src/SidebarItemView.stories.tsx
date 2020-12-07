import React from "react"
import { MemoryRouter } from "react-router"
import PathBuilder from "./PathBuilder"
import SidebarItem from "./SidebarItem"
import { SidebarItemView } from "./SidebarResources"
import { oneResourceNoAlerts } from "./testdata"
import { ResourceStatus, ResourceView, TriggerMode } from "./types"

type Resource = Proto.webviewResource
let pathBuilder = PathBuilder.forTesting("localhost", "/")

function ItemWrapper(props: { children: React.ReactNode }) {
  return (
    <MemoryRouter initialEntries={["/"]}>
      <div style={{ width: "336px" }}>{props.children}</div>
    </MemoryRouter>
  )
}

type optionFn = (item: SidebarItem) => void

function withName(n: string): optionFn {
  return (item: SidebarItem) => {
    item.name = n
  }
}

function withStatus(status: ResourceStatus): optionFn {
  return (item: SidebarItem) => {
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
  }
}

function withManualTrigger(): optionFn {
  return (item: SidebarItem) => {
    item.triggerMode = TriggerMode.TriggerModeManualIncludingInitial
    item.hasPendingChanges = true
  }
}

function itemView(...options: optionFn[]) {
  let item = new SidebarItem(oneResourceNoAlerts())
  options.forEach((option) => option(item))
  return (
    <ItemWrapper>
      <SidebarItemView
        item={item}
        selected={false}
        renderPin={true}
        resourceView={ResourceView.Log}
        pathBuilder={pathBuilder}
      />
    </ItemWrapper>
  )
}

export default {
  title: "SidebarItemView",
}

export const OneItemBuilding = () =>
  itemView(withStatus(ResourceStatus.Building))

export const OneItemPending = () => itemView(withStatus(ResourceStatus.Pending))

export const OneItemHealthy = () => itemView(withStatus(ResourceStatus.Healthy))

export const OneItemUnhealthy = () =>
  itemView(withStatus(ResourceStatus.Unhealthy))

export const OneItemWarning = () => itemView(withStatus(ResourceStatus.Warning))

export const OneItemNone = () => itemView(withStatus(ResourceStatus.None))

export const OneItemTrigger = () =>
  itemView(withStatus(ResourceStatus.Pending), withManualTrigger())

export const OneItemLongName = () =>
  itemView(withName("longnamelongnameverylongname"))
