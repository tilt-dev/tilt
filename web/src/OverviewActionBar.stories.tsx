import { createMemoryHistory } from "history"
import React from "react"
import { Router } from "react-router"
import { FilterLevel, FilterSource, useFilterSet } from "./logfilters"
import OverviewActionBar from "./OverviewActionBar"
import { StarredResourceMemoryProvider } from "./StarredResourcesContext"
import { oneResource } from "./testdata"

type Resource = Proto.webviewResource

export default {
  title: "New UI/Log View/OverviewActionBar",
  decorators: [
    (Story: any, context: any) => {
      let level = context?.args?.level || ""
      let source = context?.args?.source || ""
      let history = createMemoryHistory()
      history.location.search = `?level=${level}&source=${source}`
      return (
        <Router history={history}>
          <StarredResourceMemoryProvider>
            <div style={{ margin: "-1rem", height: "80vh" }}>
              <Story {...context.args} />
            </div>
          </StarredResourceMemoryProvider>
        </Router>
      )
    },
  ],
  argTypes: {
    source: {
      control: {
        type: "select",
        options: [FilterSource.all, FilterSource.build, FilterSource.runtime],
      },
    },
    level: {
      control: {
        type: "select",
        options: [FilterLevel.all, FilterLevel.warn, FilterLevel.error],
      },
    },
  },
}

let defaultFilter = { source: FilterSource.all, level: FilterLevel.all }

export const OverflowTextBar = () => {
  let filterSet = useFilterSet()
  let res = oneResource()
  res.endpointLinks = [
    { url: "http://my-pod-grafana-long-service-name-deadbeef:4001" },
    { url: "http://my-pod-grafana-long-service-name-deadbeef:4002" },
  ]
  res.podID = "my-pod-grafana-long-service-name-deadbeef"
  return <OverviewActionBar resource={res} filterSet={filterSet} />
}

export const FullBar = () => {
  let filterSet = useFilterSet()
  let res = oneResource()
  res.endpointLinks = [
    { url: "http://localhost:4001" },
    { url: "http://localhost:4002" },
  ]
  res.podID = "my-pod-deadbeef"
  return <OverviewActionBar resource={res} filterSet={filterSet} />
}

export const EmptyBar = () => {
  let filterSet = useFilterSet()
  let res = oneResource()
  res.endpointLinks = []
  res.podID = ""
  return <OverviewActionBar resource={res} filterSet={filterSet} />
}
