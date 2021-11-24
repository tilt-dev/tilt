import { mount } from "enzyme"
import React from "react"
import Features, { FeaturesProvider, Flag } from "./feature"
import { LogAlertIndex } from "./LogStore"
import PathBuilder from "./PathBuilder"
import SidebarItem from "./SidebarItem"
import SidebarItemView, {
  DisabledSidebarItemView,
  EnabledSidebarItemView,
} from "./SidebarItemView"
import { oneResourceNoAlerts, TestResourceOptions } from "./testdata"
import { ResourceView } from "./types"

const PATH_BUILDER = PathBuilder.forTesting("localhost", "/")
const LOG_ALERT_INDEX: LogAlertIndex = { alertsForSpanId: () => [] }

// Note: this test wrapper can be refactored to take
// more parameters as more tests are added to this suite
const SidebarItemViewTestWrapper = ({
  item,
  disableResourcesEnabled,
}: {
  item: SidebarItem
  disableResourcesEnabled?: boolean
}) => {
  const features = new Features({
    [Flag.DisableResources]: disableResourcesEnabled ?? true,
  })
  return (
    <FeaturesProvider value={features}>
      <SidebarItemView
        item={item}
        selected={false}
        resourceView={ResourceView.Log}
        pathBuilder={PATH_BUILDER}
        groupView={false}
      />
    </FeaturesProvider>
  )
}

const oneSidebarItem = (options: TestResourceOptions) => {
  return new SidebarItem(oneResourceNoAlerts(options), LOG_ALERT_INDEX)
}

describe("SidebarItemView", () => {
  describe("when `disable_resources` flag is NOT enabled", () => {
    it("does NOT display a disabled resource", () => {
      const item = oneSidebarItem({ disabled: true })
      const wrapper = mount(
        <SidebarItemViewTestWrapper
          item={item}
          disableResourcesEnabled={false}
        />
      )
      expect(wrapper.find(DisabledSidebarItemView).length).toBe(0)
    })

    it("does render an enabled resource with enabled view", () => {
      const item = oneSidebarItem({ disabled: false })
      const wrapper = mount(
        <SidebarItemViewTestWrapper
          item={item}
          disableResourcesEnabled={false}
        />
      )
      expect(wrapper.find(EnabledSidebarItemView).length).toBe(1)
    })
  })

  describe("when `disable_resources` flag is enabled", () => {
    it("does display a disabled resource with disabled view", () => {
      const item = oneSidebarItem({ disabled: true })
      const wrapper = mount(
        <SidebarItemViewTestWrapper
          item={item}
          disableResourcesEnabled={true}
        />
      )
      expect(wrapper.find(DisabledSidebarItemView).length).toBe(1)
    })

    it("does render an enabled resource with enabled view", () => {
      const item = oneSidebarItem({ disabled: false })
      const wrapper = mount(
        <SidebarItemViewTestWrapper
          item={item}
          disableResourcesEnabled={true}
        />
      )
      expect(wrapper.find(EnabledSidebarItemView).length).toBe(1)
    })
  })
})
