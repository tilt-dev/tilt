import React from "react"
import { MemoryRouter } from "react-router"
import PathBuilder from "./PathBuilder"
import { ResourceNavContextProvider } from "./ResourceNav"
import SidebarItem from "./SidebarItem"
import SidebarItemView, { SidebarItemViewProps } from "./SidebarItemView"
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
  let resourceNav = {
    selectedResource: "",
    invalidResource: "",
    openResource: (name: string) => {},
  }
  return (
    <MemoryRouter initialEntries={["/"]}>
      <ResourceNavContextProvider value={resourceNav}>
        <div style={{ width: "336px", margin: "100px" }}>{props.children}</div>
      </ResourceNavContextProvider>
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
    item.triggerMode = TriggerMode.TriggerModeManual
    item.hasPendingChanges = true
  }
}

function withQueuedTrigger(): optionFn {
  return (props: SidebarItemViewProps) => {
    let item = props.item
    item.triggerMode = TriggerMode.TriggerModeManual
    item.hasPendingChanges = true
    item.queued = true
  }
}

type Args = { selected: boolean }

function withArgs(args: Args): optionFn {
  return (props: SidebarItemViewProps) => {
    props.selected = args.selected
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
  title: "Legacy UI/SidebarItemView",
  args: { selected: false },
}

export const OneItemBuilding = (args: Args) =>
  itemView(withArgs(args), withStatus(ResourceStatus.Building))

export const OneItemPending = (args: Args) =>
  itemView(withArgs(args), withStatus(ResourceStatus.Pending))

export const OneItemHealthy = (args: Args) =>
  itemView(withArgs(args), withStatus(ResourceStatus.Healthy))

export const OneItemUnhealthy = (args: Args) =>
  itemView(withArgs(args), withStatus(ResourceStatus.Unhealthy))

export const OneItemWarning = (args: Args) =>
  itemView(withArgs(args), withStatus(ResourceStatus.Warning))

export const OneItemNone = (args: Args) =>
  itemView(withArgs(args), withStatus(ResourceStatus.None))

export const OneItemTrigger = (args: Args) =>
  itemView(
    withArgs(args),
    withStatus(ResourceStatus.Pending),
    withManualTrigger()
  )

export const OneItemQueuedTrigger = (args: Args) =>
  itemView(
    withArgs(args),
    withStatus(ResourceStatus.Pending),
    withQueuedTrigger()
  )

export const OneItemLongName = (args: Args) =>
  itemView(withArgs(args), withName("longnamelongnameverylongname"))

export const Tiltfile = (args: Args) =>
  itemView(
    withArgs(args),
    withName(ResourceName.tiltfile),
    withBuildStatusOnly(ResourceStatus.Healthy)
  )
