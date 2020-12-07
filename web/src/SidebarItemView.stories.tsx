import { storiesOf } from "@storybook/react"
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

storiesOf("SidebarItemView", module)
  .add("one-item-building", () => itemView(withStatus(ResourceStatus.Building)))
  .add("one-item-pending", () => itemView(withStatus(ResourceStatus.Pending)))
  .add("one-item-healthy", () => itemView(withStatus(ResourceStatus.Healthy)))
  .add("one-item-unhealthy", () =>
    itemView(withStatus(ResourceStatus.Unhealthy))
  )
  .add("one-item-warning", () => itemView(withStatus(ResourceStatus.Warning)))
  .add("one-item-none", () => itemView(withStatus(ResourceStatus.None)))
  .add("one-item-trigger", () =>
    itemView(withStatus(ResourceStatus.Pending), withManualTrigger())
  )
  .add("one-item-long-name", () =>
    itemView(withName("longnamelongnameverylongname"))
  )
