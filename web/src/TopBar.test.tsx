import React from "react"
import renderer from "react-test-renderer"
import { MemoryRouter } from "react-router"
import { ResourceView } from "./types"
import TopBar from "./TopBar"

const fakeHandleOpenModal = () => {}

it("shows snapshot url", () => {
  const tree = renderer
    .create(
      <MemoryRouter>
        <TopBar
          logUrl="/r/foo"
          alertsUrl="/r/foo/alerts"
          facetsUrl={null}
          resourceView={ResourceView.Alerts}
          numberOfAlerts={1}
          showSnapshotButton={true}
          handleOpenModal={fakeHandleOpenModal}
          highlight={null}
        />
      </MemoryRouter>
    )
    .toJSON()

  expect(tree).toMatchSnapshot()
})

it("doesn't render snapshot button if it's a snapshot", () => {
  const tree = renderer
    .create(
      <MemoryRouter>
        <TopBar
          logUrl="/r/foo"
          alertsUrl="/r/foo/alerts"
          facetsUrl={null}
          resourceView={ResourceView.Alerts}
          numberOfAlerts={1}
          showSnapshotButton={false}
          handleOpenModal={fakeHandleOpenModal}
          highlight={null}
        />
      </MemoryRouter>
    )
    .toJSON()

  expect(tree).toMatchSnapshot()
})
