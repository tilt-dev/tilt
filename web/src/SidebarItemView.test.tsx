import { render, RenderOptions, screen } from "@testing-library/react"
import React from "react"
import Features, { FeaturesTestProvider } from "./feature"
import { LogAlertIndex } from "./LogStore"
import PathBuilder from "./PathBuilder"
import SidebarItem from "./SidebarItem"
import SidebarItemView from "./SidebarItemView"
import { oneResource, TestResourceOptions } from "./testdata"
import { ResourceView } from "./types"

const PATH_BUILDER = PathBuilder.forTesting("localhost", "/")
const LOG_ALERT_INDEX: LogAlertIndex = { alertsForSpanId: () => [] }

function customRender(sidebarItem: SidebarItem, options?: RenderOptions) {
  const features = new Features(null)
  return render(
    <SidebarItemView
      item={sidebarItem}
      selected={false}
      resourceView={ResourceView.Log}
      pathBuilder={PATH_BUILDER}
      groupView={false}
    />,
    {
      wrapper: ({ children }) => (
        <FeaturesTestProvider value={features}>{children}</FeaturesTestProvider>
      ),
      ...options,
    }
  )
}

const oneSidebarItem = (options: TestResourceOptions) => {
  return new SidebarItem(oneResource(options), LOG_ALERT_INDEX)
}

describe("SidebarItemView", () => {
  it("does display a disabled resource with disabled view", () => {
    const item = oneSidebarItem({ disabled: true })
    customRender(item)

    expect(screen.getByText(item.name)).toBeInTheDocument()
    expect(screen.getByRole("link", { name: item.name })).toBeInTheDocument()
    expect(screen.getByRole("button", { name: /star/i })).toBeInTheDocument()
    expect(screen.queryByLabelText("Trigger update")).toBeNull()
  })

  it("does render an enabled resource with enabled view", () => {
    const item = oneSidebarItem({ disabled: false })
    customRender(item)

    expect(screen.getByText(item.name)).toBeInTheDocument()
    expect(
      screen.getAllByRole("button", { name: /star/i })[0]
    ).toBeInTheDocument()
    expect(screen.getByLabelText("Trigger update")).toBeInTheDocument()
  })
})
