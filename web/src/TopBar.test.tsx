import React from "react"
import renderer from "react-test-renderer"
import { MemoryRouter } from "react-router"
import { ResourceView } from "./types"
import TopBar from "./TopBar"

const fakeHandleOpenModal = () => {}

it("shows sail share button", () => {
  const tree = renderer
    .create(
      <MemoryRouter>
        <TopBar
          logUrl="/r/foo"
          previewUrl="/r/foo/preview"
          alertsUrl="/r/foo/alerts"
          resourceView={ResourceView.Alerts}
          sailEnabled={true}
          sailUrl=""
          numberOfAlerts={0}
          showSnapshotButton={false}
          snapshotOwner={null}
          handleOpenModal={fakeHandleOpenModal}
        />
      </MemoryRouter>
    )
    .toJSON()

  expect(tree).toMatchSnapshot()
})

it("shows sail url", () => {
  const tree = renderer
    .create(
      <MemoryRouter>
        <TopBar
          logUrl="/r/foo"
          previewUrl="/r/foo/preview"
          alertsUrl="/r/foo/alerts"
          resourceView={ResourceView.Alerts}
          sailEnabled={true}
          sailUrl="www.sail.dev/xyz"
          numberOfAlerts={1}
          showSnapshotButton={false}
          snapshotOwner={null}
          handleOpenModal={fakeHandleOpenModal}
        />
      </MemoryRouter>
    )
    .toJSON()

  expect(tree).toMatchSnapshot()
})

it("shows snapshot url", () => {
  const tree = renderer
    .create(
      <MemoryRouter>
        <TopBar
          logUrl="/r/foo"
          previewUrl="/r/foo/preview"
          alertsUrl="/r/foo/alerts"
          resourceView={ResourceView.Alerts}
          sailEnabled={false}
          sailUrl="www.sail.dev/xyz"
          numberOfAlerts={1}
          showSnapshotButton={true}
          snapshotOwner={null}
          handleOpenModal={fakeHandleOpenModal}
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
                    previewUrl="/r/foo/preview"
                    alertsUrl="/r/foo/alerts"
                    resourceView={ResourceView.Alerts}
                    sailEnabled={false}
                    sailUrl="www.sail.dev/xyz"
                    numberOfAlerts={1}
                    showSnapshotButton={false}
                    snapshotOwner="foo"
                    handleOpenModal={fakeHandleOpenModal}
                />
            </MemoryRouter>
        )
        .toJSON()

    expect(tree).toMatchSnapshot()
})
