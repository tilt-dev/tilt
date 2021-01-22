import React from "react"
import { MemoryRouter } from "react-router"
import OverviewActionBar from "./OverviewActionBar"
import { SidebarPinMemoryProvider } from "./SidebarPin"
import { oneResource } from "./testdata"

type Resource = Proto.webviewResource

export default {
  title: "OverviewActionBar",
  decorators: [
    (Story: any) => (
      <MemoryRouter initialEntries={["/"]}>
        <SidebarPinMemoryProvider>
          <div style={{ margin: "-1rem", height: "80vh" }}>
            <Story />
          </div>
        </SidebarPinMemoryProvider>
      </MemoryRouter>
    ),
  ],
}

export const OverflowTextBar = () => {
  let res = oneResource()
  res.endpointLinks = [
    { url: "http://my-pod-grafana-long-service-name-deadbeef:4001" },
    { url: "http://my-pod-grafana-long-service-name-deadbeef:4002" },
  ]
  res.podID = "my-pod-grafana-long-service-name-deadbeef"
  return <OverviewActionBar resource={res} />
}

export const FullBar = () => {
  let res = oneResource()
  res.endpointLinks = [
    { url: "http://localhost:4001" },
    { url: "http://localhost:4002" },
  ]
  res.podID = "my-pod-deadbeef"
  return <OverviewActionBar resource={res} />
}

export const EmptyBar = () => {
  let res = oneResource()
  res.endpointLinks = []
  res.podID = ""
  return <OverviewActionBar resource={res} />
}
