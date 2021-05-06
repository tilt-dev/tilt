import { createMemoryHistory } from "history"
import React from "react"
import { Router } from "react-router"
import { FilterLevel, FilterSource, useFilterSet } from "./logfilters"
import OverviewActionBar from "./OverviewActionBar"
import { StarredResourceMemoryProvider } from "./StarredResourcesContext"
import { oneResource } from "./testdata"

type UIResource = Proto.v1alpha1UIResource

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
  res.status = res.status || {}
  res.status.endpointLinks = [
    { url: "http://my-pod-grafana-long-service-name-deadbeef:4001" },
    { url: "http://my-pod-grafana-long-service-name-deadbeef:4002" },
  ]
  res.status.k8sResourceInfo = {
    podName: "my-pod-grafana-long-service-name-deadbeef",
  }
  return <OverviewActionBar resource={res} filterSet={filterSet} />
}

export const FullBar = () => {
  let filterSet = useFilterSet()
  let res = oneResource()
  res.status = res.status || {}
  res.status.endpointLinks = [
    { url: "http://localhost:4001" },
    { url: "http://localhost:4002" },
  ]
  res.status.k8sResourceInfo = { podName: "my-pod-deadbeef" }
  return <OverviewActionBar resource={res} filterSet={filterSet} />
}

export const EmptyBar = () => {
  let filterSet = useFilterSet()
  let res = oneResource()
  res.status = res.status || {}
  res.status.endpointLinks = []
  res.status.k8sResourceInfo = {}
  return <OverviewActionBar resource={res} filterSet={filterSet} />
}
