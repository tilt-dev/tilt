import React from "react"
import { MemoryRouter } from "react-router"
import PathBuilder from "./PathBuilder"
import SidebarItem from "./SidebarItem"
import SidebarItemView, {
  SidebarItemAll,
  SidebarItemViewProps,
} from "./SidebarItemView"
import { LegacyNavProvider } from "./TabNav"
import { oneResourceNoAlerts } from "./testdata"
import {
  ResourceName,
  ResourceStatus,
  ResourceView,
  TriggerMode,
} from "./types"

type Resource = Proto.webviewResource
let pathBuilder = PathBuilder.forTesting("localhost", "/")

function ItemWrapper(props: { children: React.ReactNode }) {
  return (
    <MemoryRouter initialEntries={["/"]}>
      <LegacyNavProvider resourceView={ResourceView.Log}>
        <div style={{ width: "336px", margin: "100px" }}>{props.children}</div>
      </LegacyNavProvider>
    </MemoryRouter>
  )
}

type optionFn = (item: SidebarItemViewProps) => void

function withName(n: string): optionFn {
  return (props: SidebarItemViewProps) => {
    let item = props.item
    item.name = n
  }
}

function withBuildStatusOnly(status: ResourceStatus): optionFn {
  return (props: SidebarItemViewProps) => {
    let item = props.item
    item.buildStatus = status
    item.runtimeStatus = ResourceStatus.None
    if (status === ResourceStatus.Building) {
      item.currentBuildStartTime = new Date(Date.now() - 1).toISOString()
    }
    if (
      status === ResourceStatus.Unhealthy ||
      status === ResourceStatus.Warning
    ) {
      item.buildAlertCount = 1
    }
  }
}

function withStatus(status: ResourceStatus): optionFn {
  return (props: SidebarItemViewProps) => {
    let item = props.item
    item.buildStatus = status
    item.runtimeStatus = status
    if (status === ResourceStatus.Building) {
      item.currentBuildStartTime = new Date(Date.now() - 1).toISOString()
    }
    if (
      status === ResourceStatus.Unhealthy ||
      status === ResourceStatus.Warning
    ) {
      item.buildAlertCount = 1
      item.runtimeAlertCount = 1
    }
  }
}

function withManualTrigger(): optionFn {
  return (props: SidebarItemViewProps) => {
    let item = props.item
    item.triggerMode = TriggerMode.TriggerModeManualIncludingInitial
    item.hasPendingChanges = true
  }
}

function withSelected(v: boolean): optionFn {
  return (props: SidebarItemViewProps) => {
    props.selected = v
  }
}

function itemView(...options: optionFn[]) {
  let item = new SidebarItem(oneResourceNoAlerts())
  let props = {
    item: item,
    selected: false,
    resourceView: ResourceView.Log,
    pathBuilder: pathBuilder,
  }
  options.forEach((option) => option(props))
  return (
    <ItemWrapper>
      <SidebarItemView {...props} />
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

export const OneItemHealthySelected = () =>
  itemView(withStatus(ResourceStatus.Healthy), withSelected(true))

export const OneItemUnhealthy = () =>
  itemView(withStatus(ResourceStatus.Unhealthy))

export const OneItemWarning = () => itemView(withStatus(ResourceStatus.Warning))

export const OneItemNone = () => itemView(withStatus(ResourceStatus.None))

export const OneItemTrigger = () =>
  itemView(withStatus(ResourceStatus.Pending), withManualTrigger())

export const OneItemLongName = () =>
  itemView(withName("longnamelongnameverylongname"))

export const Tiltfile = () =>
  itemView(
    withName(ResourceName.tiltfile),
    withBuildStatusOnly(ResourceStatus.Healthy)
  )

export const AllItemSelected = () => {
  return (
    <ItemWrapper>
      <SidebarItemAll nothingSelected={true} totalAlerts={1} />
    </ItemWrapper>
  )
}

export const AllItemUnselected = () => {
  return (
    <ItemWrapper>
      <SidebarItemAll nothingSelected={false} totalAlerts={1} />
    </ItemWrapper>
  )
}
