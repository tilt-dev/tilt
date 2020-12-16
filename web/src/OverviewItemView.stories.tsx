import React from "react"
import { MemoryRouter } from "react-router"
import OverviewItemView, {
  OverviewItem,
  OverviewItemViewProps,
} from "./OverviewItemView"
import PathBuilder from "./PathBuilder"
import { oneResourceNoAlerts } from "./testdata"
import { ResourceStatus, TriggerMode } from "./types"

type Resource = Proto.webviewResource
let pathBuilder = PathBuilder.forTesting("localhost", "/")

function ItemWrapper(props: { children: React.ReactNode }) {
  return (
    <MemoryRouter initialEntries={["/"]}>
      <div style={{ width: "336px", margin: "100px" }}>{props.children}</div>
    </MemoryRouter>
  )
}

type optionFn = (item: OverviewItemViewProps) => void

function withName(n: string): optionFn {
  return (props: OverviewItemViewProps) => {
    let item = props.item
    item.name = n
  }
}

function withBuildStatusOnly(status: ResourceStatus): optionFn {
  return (props: OverviewItemViewProps) => {
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
  return (props: OverviewItemViewProps) => {
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
  return (props: OverviewItemViewProps) => {
    let item = props.item
    item.triggerMode = TriggerMode.TriggerModeManualIncludingInitial
    item.hasPendingChanges = true
  }
}

function itemView(...options: optionFn[]) {
  let item = new OverviewItem(oneResourceNoAlerts())
  let props = {
    item: item,
    pathBuilder: pathBuilder,
  }
  options.forEach((option) => option(props))
  return (
    <ItemWrapper>
      <OverviewItemView {...props} />
    </ItemWrapper>
  )
}

export default {
  title: "OverviewItemView",
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

export const Tiltfile = () =>
  itemView(withName("(Tiltfile)"), withBuildStatusOnly(ResourceStatus.Healthy))
