import React from "react"
import { MemoryRouter } from "react-router"
import s from "styled-components"
import SecondaryNav from "./SecondaryNav"
import { ResourceView } from "./types"

let BG = s.div`
  background-color: #001b20;
  width: 100%;
  padding-top: 32px;
`

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
          metricsUrl=""
        />
      </BG>
    </MemoryRouter>
  )
}

function topBarWithMetrics() {
  return (
    <MemoryRouter>
      <BG>
        <SecondaryNav
          logUrl="/"
          alertsUrl="/alerts"
          resourceView={ResourceView.Metrics}
          numberOfAlerts={1}
          facetsUrl=""
          traceNav={null}
          metricsUrl="/metrics"
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
          metricsUrl=""
        />
      </BG>
    </MemoryRouter>
  )
}

function traceNavFirst() {
  const traceNav = {
    count: 3,
    current: {
      url: "/r/foo/trace/build:1",
      index: 0,
    },
    next: {
      url: "/r/foo/trace/build:2",
      index: 1,
    },
  }
  return (
    <MemoryRouter>
      <BG>
        <SecondaryNav
          logUrl="/r/foo"
          alertsUrl="/r/foo/alerts"
          resourceView={ResourceView.Trace}
          numberOfAlerts={0}
          facetsUrl="/r/foo/facets"
          traceNav={traceNav}
          metricsUrl=""
        />
      </BG>
    </MemoryRouter>
  )
}
function traceNavLast() {
  const traceNav = {
    count: 3,
    prev: {
      url: "/r/foo/trace/build:2",
      index: 1,
    },
    current: {
      url: "/r/foo/trace/build:1",
      index: 2,
    },
  }
  return (
    <MemoryRouter>
      <BG>
        <SecondaryNav
          logUrl="/r/foo"
          alertsUrl="/r/foo/alerts"
          resourceView={ResourceView.Trace}
          numberOfAlerts={0}
          facetsUrl="/r/foo/facets"
          traceNav={traceNav}
          metricsUrl=""
        />
      </BG>
    </MemoryRouter>
  )
}

export default {
  title: "Legacy UI/SecondaryNav",
}

export const Default = topBarDefault

export const _Metrics = topBarWithMetrics

export const Team = topBarTeam

export const TraceNavFirst = traceNavFirst

export const TraceNavLast = traceNavLast
