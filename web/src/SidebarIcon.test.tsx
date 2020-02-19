import React from "react"
import { mount, shallow } from "enzyme"
import SidebarIcon, { IconType } from "./SidebarIcon"
import { ResourceStatus } from "./types"
import { Color } from "./style-helpers"

const cases: Array<[string, ResourceStatus, Color]> = [
  ["isPending", ResourceStatus.Pending, Color.white],
  ["isHealthy", ResourceStatus.Healthy, Color.green],
  ["isUnhealthy", ResourceStatus.Unhealthy, Color.red],
  ["isBuilding", ResourceStatus.Building, Color.white],
  ["isWarning", ResourceStatus.Warning, Color.yellow],
  ["isNone", ResourceStatus.None, Color.white],
]

test.each(cases)("renders correctly - %s", (className, status, color) => {
  // TODO(han) - need to check that background-color is as expected
  // Since we're using styled-components, we should test individual style rules with
  // jest-styled-components, which requires an upgrade to styled-components v5
  const wrapper = shallow(<SidebarIcon status={status} alertCount={0} />)
  expect(wrapper).toMatchSnapshot()
})
