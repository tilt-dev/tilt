import React from "react"
import { storiesOf } from "@storybook/react"
import SecondaryNav from "./SecondaryNav"
import { MemoryRouter } from "react-router"
import { ResourceView } from "./types"
import s from "styled-components"

let BG = s.div`
  background-color: #001b20;
  width: 100%;
  padding-top: 32px;
`

function openModal() {
  console.log("openModal")
}

function topBarDefault() {
  return (
    <MemoryRouter>
      <BG>
        <SecondaryNav
          logUrl="/r/foo"
          alertsUrl="/r/foo/alerts"
          resourceView={ResourceView.Alerts}
          numberOfAlerts={1}
          facetsUrl="/r/foo/facets"
          traceNav={null}
        />
      </BG>
    </MemoryRouter>
  )
}

function topBarTeam() {
  return (
    <MemoryRouter>
      <BG>
        <SecondaryNav
          logUrl="/r/foo"
          alertsUrl="/r/foo/alerts"
          resourceView={ResourceView.Alerts}
          numberOfAlerts={1}
          facetsUrl="/r/foo/facets"
          traceNav={null}
        />
      </BG>
    </MemoryRouter>
  )
}

function traceTab() {
  return (
    <MemoryRouter>
      <BG>
        <SecondaryNav
          logUrl="/r/foo"
          alertsUrl="/r/foo/alerts"
          resourceView={ResourceView.Alerts}
          numberOfAlerts={1}
          facetsUrl="/r/foo/facets"
          traceNav={null}
        />
      </BG>
    </MemoryRouter>
  )
}

storiesOf("SecondaryNav", module)
  .add("default", topBarDefault)
  .add("team", topBarTeam)
