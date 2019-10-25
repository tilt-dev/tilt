import React from "react"
import { storiesOf } from "@storybook/react"
import TopBar from "./TopBar"
import { MemoryRouter } from "react-router"
import { ResourceView } from "./types"

function openModal() {
  console.log("openModal")
}

function topBarDefault() {
  return (
    <MemoryRouter>
      <TopBar
        logUrl="/r/foo"
        alertsUrl="/r/foo/alerts"
        resourceView={ResourceView.Alerts}
        numberOfAlerts={1}
        showSnapshotButton={true}
        snapshotOwner={null}
        handleOpenModal={openModal}
        highlight={null}
        teamSnapshotsUrl={""}
        teamUpdatesUrl={""}
        facetsUrl="/r/foo/facets"
      />
    </MemoryRouter>
  )
}

function topBarTeam() {
  return (
    <MemoryRouter>
      <TopBar
        logUrl="/r/foo"
        alertsUrl="/r/foo/alerts"
        resourceView={ResourceView.Alerts}
        numberOfAlerts={1}
        showSnapshotButton={true}
        snapshotOwner={null}
        handleOpenModal={openModal}
        highlight={null}
        teamSnapshotsUrl={"/team/snapshots"}
        teamUpdatesUrl={"/team/updates"}
        facetsUrl="/r/foo/facets"
      />
    </MemoryRouter>
  )
}

storiesOf("TopBar", module)
  .add("default", topBarDefault)
  .add("team", topBarTeam)
