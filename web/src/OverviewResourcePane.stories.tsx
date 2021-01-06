import React from "react"
import { MemoryRouter } from "react-router"
import LogStore, { LogStoreProvider } from "./LogStore"
import OverviewResourcePane from "./OverviewResourcePane"
import { SidebarPinMemoryProvider } from "./SidebarPin"
import { nResourceView, tenResourceView, twoResourceView } from "./testdata"

type Resource = Proto.webviewResource

export default {
  title: "OverviewResourcePane",
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

export const TwoResources = () => (
  <OverviewResourcePane name={"vigoda"} view={twoResourceView()} />
)

export const TenResources = () => (
  <OverviewResourcePane name={"vigoda_1"} view={tenResourceView()} />
)

export const TenResourcesWithLogStore = () => {
  let logStore = new LogStore()
  let segments = []
  for (let i = 0; i < 100; i++) {
    segments.push({
      spanId: "build:1",
      text: `Vigoda build line ${i}\n`,
      time: new Date().toString(),
    })
  }
  logStore.append({
    spans: {
      "build:1": { manifestName: "vigoda_1" },
    },
    segments: segments,
  })

  return (
    <LogStoreProvider value={logStore}>
      <OverviewResourcePane name={"vigoda_1"} view={tenResourceView()} />
    </LogStoreProvider>
  )
}

export const OneHundredResources = () => (
  <OverviewResourcePane name={"vigoda_1"} view={nResourceView(100)} />
)

export const NotFound = () => (
  <OverviewResourcePane name={"does-not-exist"} view={twoResourceView()} />
)
