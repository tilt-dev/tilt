import React from "react"
import { MemoryRouter } from "react-router"
import OverviewResourceDetails from "./OverviewResourceDetails"
import { oneResource } from "./testdata"

type Resource = Proto.webviewResource

export default {
  title: "OverviewResourceDetails",
  decorators: [
    (Story: any) => (
      <MemoryRouter initialEntries={["/"]}>
        <div style={{ margin: "-1rem", height: "80vh" }}>
          <Story />
        </div>
      </MemoryRouter>
    ),
  ],
}

export const FullBar = () => {
  let res = oneResource()
  res.endpointLinks = [
    { url: "http://localhost:4001" },
    { url: "http://localhost:4002" },
  ]
  res.podID = "my-pod-deadbeef"
  return <OverviewResourceDetails resource={res} />
}
export const EmptyBar = () => {
  let res = oneResource()
  res.endpointLinks = []
  res.podID = ""
  return <OverviewResourceDetails resource={res} />
}
