import React from "react"
import { storiesOf } from "@storybook/react"
import SecondaryNav from "./SecondaryNav"
import { MemoryRouter } from "react-router"
import { ResourceView } from "./types"

function openModal() {
  console.log("openModal")
}

function topBarDefault() {
  return (
    <MemoryRouter>
      <SecondaryNav
        logUrl="/r/foo"
        alertsUrl="/r/foo/alerts"
        resourceView={ResourceView.Alerts}
        numberOfAlerts={1}
        facetsUrl="/r/foo/facets"
        traceUrl={""}
        span={""}
      />
    </MemoryRouter>
  )
}

function topBarTeam() {
  return (
    <MemoryRouter>
      <SecondaryNav
        logUrl="/r/foo"
        alertsUrl="/r/foo/alerts"
        resourceView={ResourceView.Alerts}
        numberOfAlerts={1}
        facetsUrl="/r/foo/facets"
        traceUrl={""}
        span={""}
      />
    </MemoryRouter>
  )
}

storiesOf("SecondaryNav", module)
  .add("default", topBarDefault)
  .add("team", topBarTeam)
