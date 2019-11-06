import React from "react"
import { mount } from "enzyme"
import SidebarIcon, { IconType } from "./SidebarIcon"
import { ResourceStatus } from "./types"
import { Color } from "./constants"

type Ignore = boolean

const cases: Array<
  [string, ResourceStatus, Color | Ignore, IconType | Ignore]
> = [
  ["pending", ResourceStatus.Pending, false, IconType.StatusPending],
  ["healthy", ResourceStatus.Healthy, Color.green, IconType.StatusDefault],
  ["unhealthy", ResourceStatus.Unhealthy, Color.red, IconType.StatusDefault],
  ["building", ResourceStatus.Building, false, IconType.StatusBuilding],
  ["none", ResourceStatus.None, Color.gray, IconType.StatusDefault],
]

test.each(cases)("%s", (_, status, fillColor, iconType) => {
  const root = mount(<SidebarIcon status={status} />)

  if (fillColor !== false) {
    expect(root.find(`svg[fill="${fillColor}"]`)).toHaveLength(1)
  }

  if (iconType !== false) {
    let path = `svg.${iconType}`
    expect(root.find(path)).toHaveLength(1)
  }
})
