import React from "react"
import renderer from "react-test-renderer"
import { MemoryRouter } from "react-router"
import { Resource, ResourceView, Snapshot, TiltBuild } from "./types"
import TopBar from "./TopBar"

const testState: Snapshot = {
  Message: "",
  View: null,
  IsSidebarClosed: false,
}

const fakeSendSnapshot = (snapshot: Snapshot) => void {}

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
          state={testState}
          handleSendSnapshot={fakeSendSnapshot}
          snapshotURL=""
          showSnapshot={false}
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
          state={testState}
          handleSendSnapshot={fakeSendSnapshot}
          snapshotURL=""
          showSnapshot={false}
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
          state={testState}
          handleSendSnapshot={fakeSendSnapshot}
          snapshotURL=""
          showSnapshot={true}
        />
      </MemoryRouter>
    )
    .toJSON()

  expect(tree).toMatchSnapshot()
})
