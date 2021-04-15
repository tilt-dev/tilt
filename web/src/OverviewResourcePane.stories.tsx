import React from "react"
import { MemoryRouter } from "react-router"
import LogStore, { LogStoreProvider } from "./LogStore"
import OverviewResourcePane from "./OverviewResourcePane"
import { ResourceNavProvider } from "./ResourceNav"
import { StarredResourceMemoryProvider } from "./StarredResourcesContext"
import { nResourceView, tenResourceView, twoResourceView } from "./testdata"

type Resource = Proto.webviewResource

export default {
  title: "New UI/OverviewResourcePane",
  decorators: [
    (Story: any) => (
      <MemoryRouter initialEntries={["/"]}>
        <StarredResourceMemoryProvider>
          <div style={{ margin: "-1rem", height: "80vh" }}>
            <Story />
          </div>
        </StarredResourceMemoryProvider>
      </MemoryRouter>
    ),
  ],
}

function OverviewResourcePaneHarness(props: {
  name: string
  view: Proto.webviewView
}) {
  let { name, view } = props
  let entry = name ? `/r/${props.name}/overview` : `/overview`
  let resources = view?.resources || []
  let validateResource = (name: string) =>
    resources.some((res) => res.name == name)
  return (
    <MemoryRouter initialEntries={[entry]}>
      <ResourceNavProvider validateResource={validateResource}>
        <OverviewResourcePane view={view} />
      </ResourceNavProvider>
    </MemoryRouter>
  )
}

export const TwoResources = () => (
  <OverviewResourcePaneHarness name={"vigoda"} view={twoResourceView()} />
)

export const TenResources = () => (
  <OverviewResourcePaneHarness name="vigoda_1" view={tenResourceView()} />
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
  return <OverviewResourcePaneHarness name="vigoda_1" view={view} />
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
      <OverviewResourcePaneHarness name="vigoda_1" view={tenResourceView()} />
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
      <OverviewResourcePaneHarness name={"vigoda_1"} view={tenResourceView()} />
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
      <OverviewResourcePaneHarness name="vigoda_1" view={tenResourceView()} />
    </LogStoreProvider>
  )
}

export const OneHundredResources = () => (
  <OverviewResourcePaneHarness name="vigoda_1" view={nResourceView(100)} />
)

export const NotFound = () => (
  <OverviewResourcePaneHarness name="does-not-exist" view={twoResourceView()} />
)
