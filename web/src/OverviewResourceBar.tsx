import React from "react"
import styled from "styled-components"
import { GlobalNav, GlobalNavRoot } from "./GlobalNav"
import { usePathBuilder } from "./PathBuilder"
import { AllResourceStatusSummary } from "./ResourceStatusSummary"
import { useSnapshotAction } from "./snapshot"
import { SizeUnit } from "./style-helpers"
import { showUpdate } from "./UpdateDialog"

type OverviewResourceBarProps = {
  view: Proto.webviewView
}

const OverviewResourceBarRoot = styled.div`
  display: flex;
  align-items: stretch;
  padding-left: ${SizeUnit(1)};

  ${GlobalNavRoot} {
    justify-content: flex-end;
    flex-grow: 1;
    display: flex;
  }
`

export default function OverviewResourceBar(props: OverviewResourceBarProps) {
  let isSnapshot = usePathBuilder().isSnapshot()
  let snapshot = useSnapshotAction()
  let view = props.view
  let session = view?.uiSession?.status
  let runningBuild = session?.runningTiltBuild
  let suggestedVersion = session?.suggestedTiltVersion
  let resources = view?.uiResources || []

  let globalNavProps = {
    isSnapshot,
    snapshot,
    showUpdate: showUpdate(view),
    suggestedVersion,
    runningBuild,
    tiltCloudUsername: session?.tiltCloudUsername ?? "",
    tiltCloudSchemeHost: session?.tiltCloudSchemeHost ?? "",
    tiltCloudTeamID: session?.tiltCloudTeamID ?? "",
    tiltCloudTeamName: session?.tiltCloudTeamName ?? "",
  }

  return (
    <OverviewResourceBarRoot>
      <AllResourceStatusSummary resources={resources} />
      <GlobalNav {...globalNavProps} />
    </OverviewResourceBarRoot>
  )
}
