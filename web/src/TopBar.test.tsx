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
          resourceView={ResourceView.Alerts}
          numberOfAlerts={1}
          showSnapshotButton={true}
          snapshotOwner={null}
          handleOpenModal={fakeHandleOpenModal}
          highlight={null}
        />
      </MemoryRouter>
    )
    .toJSON()

  expect(tree).toMatchSnapshot()
})

it("shows snapshot owner", () => {
  const tree = renderer
    .create(
      <MemoryRouter>
        <TopBar
          logUrl="/r/foo"
          alertsUrl="/r/foo/alerts"
          resourceView={ResourceView.Alerts}
          numberOfAlerts={1}
          showSnapshotButton={false}
          snapshotOwner="foo"
          handleOpenModal={fakeHandleOpenModal}
          highlight={null}
        />
      </MemoryRouter>
    )
    .toJSON()

  expect(tree).toMatchSnapshot()
})
