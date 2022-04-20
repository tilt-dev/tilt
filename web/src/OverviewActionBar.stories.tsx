import { createMemoryHistory } from "history"
import React from "react"
import { Router } from "react-router"
import { ButtonSet } from "./ApiButton"
import { FilterLevel, FilterSource, useFilterSet } from "./logfilters"
import OverviewActionBar from "./OverviewActionBar"
import { TiltSnackbarProvider } from "./Snackbar"
import { StarredResourceMemoryProvider } from "./StarredResourcesContext"
import { disableButton, oneResource, oneUIButton } from "./testdata"

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
          <TiltSnackbarProvider>
            <StarredResourceMemoryProvider>
              <div style={{ margin: "-1rem", height: "80vh" }}>
                <Story {...context.args} />
              </div>
            </StarredResourceMemoryProvider>
          </TiltSnackbarProvider>
        </Router>
      )
    },
  ],
  argTypes: {
    source: {
      name: "Source",
      control: {
        type: "select",
        options: [FilterSource.all, FilterSource.build, FilterSource.runtime],
      },
    },
    level: {
      name: "Level",
      control: {
        type: "select",
        options: [FilterLevel.all, FilterLevel.warn, FilterLevel.error],
      },
    },
  },
}

export const OverflowTextBar = () => {
  let filterSet = useFilterSet()
  let res = oneResource({
    isBuilding: true,
    name: "my-grafana-long-service-name-deadbeef",
    endpoints: 2,
  })
  return <OverviewActionBar resource={res} filterSet={filterSet} />
}

export const FullBar = () => {
  let filterSet = useFilterSet()
  let res = oneResource({ isBuilding: true, name: "my-deadbeef", endpoints: 2 })
  let buttons: ButtonSet = {
    default: [oneUIButton({ buttonName: "button2", componentID: "vigoda" })],
    toggleDisable: disableButton("vigoda", true),
  }
  return (
    <OverviewActionBar resource={res} filterSet={filterSet} buttons={buttons} />
  )
}

export const EmptyBar = () => {
  let filterSet = useFilterSet()
  let res = oneResource({ isBuilding: true, endpoints: 0 })
  res.status = res.status || {}
  res.status.k8sResourceInfo = {}
  return <OverviewActionBar resource={res} filterSet={filterSet} />
}
