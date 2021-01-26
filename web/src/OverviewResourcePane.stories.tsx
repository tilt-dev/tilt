import React from "react"
import { MemoryRouter } from "react-router"
import LogStore, { LogStoreProvider } from "./LogStore"
import OverviewResourcePane from "./OverviewResourcePane"
import { SidebarPinMemoryProvider } from "./SidebarPin"
import { OverviewNavProvider } from "./TabNav"
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
  <OverviewNavProvider candidateTabForTesting={"vigoda"}>
    <OverviewResourcePane view={twoResourceView()} />
  </OverviewNavProvider>
)

export const TenResources = () => (
  <OverviewNavProvider candidateTabForTesting={"vigoda_1"}>
    <OverviewResourcePane view={tenResourceView()} />
  </OverviewNavProvider>
)

export const FullResourceBar = () => {
  let view = tenResourceView()
  let res = view.resources[1]
  res.endpointLinks = [
    { url: "http://localhost:4001" },
    { url: "http://localhost:4002" },
    { url: "http://localhost:4003" },
  ]
  res.podID = "my-pod-deadbeef"
  return (
    <OverviewNavProvider candidateTabForTesting={"vigoda_1"}>
      <OverviewResourcePane view={view} />
    </OverviewNavProvider>
  )
}

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
      <OverviewNavProvider candidateTabForTesting={"vigoda_1"}>
        <OverviewResourcePane view={tenResourceView()} />
      </OverviewNavProvider>
    </LogStoreProvider>
  )
}

export const TenResourcesWithLongLogLines = () => {
  let logStore = new LogStore()
  let segments = []
  for (let i = 0; i < 100; i++) {
    let text = []
    for (let j = 0; j < 10; j++) {
      text.push(`Vigoda build line ${i}`)
    }

    segments.push({
      spanId: "build:1",
      text: text.join(", ") + "\n",
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
      <OverviewNavProvider candidateTabForTesting={"vigoda_1"}>
        <OverviewResourcePane view={tenResourceView()} />
      </OverviewNavProvider>
    </LogStoreProvider>
  )
}

export const TenResourcesWithBuildLogAndPodLog = () => {
  let logStore = new LogStore()
  let segments = []
  for (let i = 0; i < 10; i++) {
    segments.push({
      spanId: "build:1",
      text: `Vigoda build line ${i}\n`,
      time: new Date().toString(),
    })
  }
  for (let i = 0; i < 10; i++) {
    segments.push({
      spanId: "pod:1",
      text: `Vigoda pod line ${i}\n`,
      time: new Date().toString(),
    })
  }
  logStore.append({
    spans: {
      "build:1": { manifestName: "vigoda_1" },
      "pod:1": { manifestName: "vigoda_1" },
    },
    segments: segments,
  })

  return (
    <LogStoreProvider value={logStore}>
      <OverviewNavProvider candidateTabForTesting={"vigoda_1"}>
        <OverviewResourcePane view={tenResourceView()} />
      </OverviewNavProvider>
    </LogStoreProvider>
  )
}

export const OneHundredResources = () => (
  <OverviewNavProvider candidateTabForTesting={"vigoda_1"}>
    <OverviewResourcePane view={nResourceView(100)} />
  </OverviewNavProvider>
)

export const NotFound = () => (
  <OverviewNavProvider candidateTabForTesting={"does-not-exist"}>
    <OverviewResourcePane view={twoResourceView()} />
  </OverviewNavProvider>
)
