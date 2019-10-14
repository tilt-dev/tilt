import React from "react"
import renderer from "react-test-renderer"
import TabNav from "./TabNav"
import { MemoryRouter } from "react-router"
import { ResourceView } from "./types"

it("shows logs", () => {
  const tree = renderer
    .create(
      <MemoryRouter>
        <TabNav
          logUrl="/r/foo"
          alertsUrl="/r/foo/alerts"
          resourceView={ResourceView.Log}
          numberOfAlerts={0}
        />
      </MemoryRouter>
    )
    .toJSON()

  expect(tree).toMatchSnapshot()
})

it("shows error pane", () => {
  const tree = renderer
    .create(
      <MemoryRouter>
        <TabNav
          logUrl="/r/foo"
          alertsUrl="/r/foo/alerts"
          resourceView={ResourceView.Alerts}
          numberOfAlerts={0}
        />
      </MemoryRouter>
    )
    .toJSON()

  expect(tree).toMatchSnapshot()
})

it("shows the number of errors in the error tab", () => {
  const tree = renderer
    .create(
      <MemoryRouter>
        <TabNav
          logUrl="/r/foo"
          alertsUrl="/r/foo/alerts"
          resourceView={ResourceView.Alerts}
          numberOfAlerts={27}
        />
      </MemoryRouter>
    )
    .toJSON()

  expect(tree).toMatchSnapshot()
})
