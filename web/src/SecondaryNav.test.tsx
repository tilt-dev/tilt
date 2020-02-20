import React from "react"
import renderer from "react-test-renderer"
import SecondaryNav from "./SecondaryNav"
import { MemoryRouter } from "react-router"
import { ResourceView } from "./types"

it("shows logs", () => {
  const tree = renderer
    .create(
      <MemoryRouter>
        <SecondaryNav
          logUrl="/r/foo"
          alertsUrl="/r/foo/alerts"
          traceUrl={null}
          facetsUrl={null}
          resourceView={ResourceView.Log}
          numberOfAlerts={0}
          span={""}
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
        <SecondaryNav
          logUrl="/r/foo"
          alertsUrl="/r/foo/alerts"
          facetsUrl={null}
          traceUrl={null}
          resourceView={ResourceView.Alerts}
          numberOfAlerts={0}
          span={""}
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
        <SecondaryNav
          logUrl="/r/foo"
          alertsUrl="/r/foo/alerts"
          facetsUrl={null}
          traceUrl={null}
          resourceView={ResourceView.Alerts}
          numberOfAlerts={27}
          span={""}
        />
      </MemoryRouter>
    )
    .toJSON()

  expect(tree).toMatchSnapshot()
})

it("shows a facets tab", () => {
  const tree = renderer
    .create(
      <MemoryRouter>
        <SecondaryNav
          logUrl="/r/foo"
          alertsUrl="/r/foo/alerts"
          facetsUrl="/r/foo/facets"
          traceUrl={null}
          resourceView={ResourceView.Facets}
          numberOfAlerts={0}
          span={""}
        />
      </MemoryRouter>
    )
    .toJSON()

  expect(tree).toMatchSnapshot()
})
