import React from "react"
import { MemoryRouter } from "react-router"
import OverviewItemView, {
  OverviewItem,
  OverviewItemDetails,
  OverviewItemViewProps,
} from "./OverviewItemView"
import { oneResourceNoAlerts } from "./testdata"
import { ResourceStatus, TriggerMode } from "./types"

type Resource = Proto.webviewResource

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

function withQueuedTrigger(): optionFn {
  return (props: OverviewItemViewProps) => {
    let item = props.item
    item.triggerMode = TriggerMode.TriggerModeManualIncludingInitial
    item.hasPendingChanges = true
    item.queued = true
  }
}

function itemView(...options: optionFn[]) {
  let item = new OverviewItem(oneResourceNoAlerts())
  let props = {
    item: item,
  }
  options.forEach((option) => option(props))
  return <OverviewItemView {...props} />
}

export default {
  title: "OverviewItemView",
  decorators: [
    (Story: any) => (
      <MemoryRouter initialEntries={["/"]}>
        <div style={{ width: "336px", margin: "100px" }}>
          <Story />
        </div>
      </MemoryRouter>
    ),
  ],
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

export const OneItemQueuedTrigger = () =>
  itemView(withStatus(ResourceStatus.Pending), withQueuedTrigger())

export const OneItemLongName = () =>
  itemView(withName("longnamelongnameverylongname"))

export const Tiltfile = () =>
  itemView(withName("(Tiltfile)"), withBuildStatusOnly(ResourceStatus.Healthy))

export const MinimumDetails = () => {
  let item = new OverviewItem(oneResourceNoAlerts())
  item.endpoints = []
  item.podId = ""
  return <OverviewItemDetails item={item} />
}

export const CompleteDetails = () => {
  let item = new OverviewItem(oneResourceNoAlerts())
  item.endpoints = [
    { url: "http://localhost:4001" },
    { url: "http://localhost:4002" },
  ]
  item.podId = "my-pod-deadbeef"
  return <OverviewItemDetails item={item} />
}

export const LongDetails = () => {
  let item = new OverviewItem(oneResourceNoAlerts())
  item.endpoints = [
    { url: "http://my-pod-grafana-long-service-name-deadbeef:4001" },
    { url: "http://my-pod-grafana-long-service-name-deadbeef:4002" },
  ]
  item.podId = "my-pod-grafana-long-service-name-deadbeef"
  return <OverviewItemDetails item={item} />
}
