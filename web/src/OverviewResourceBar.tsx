import React from "react"
import styled from "styled-components"
import { GlobalNav, GlobalNavRoot } from "./GlobalNav"
import { usePathBuilder } from "./PathBuilder"
import { AllResourceStatusSummary } from "./ResourceStatusSummary"
import { useSnapshotAction } from "./snapshot"
import { SizeUnit } from "./style-helpers"
import { showUpdate } from "./UpdateDialog"
import type { View } from "./webview"

type OverviewResourceBarProps = {
  view: View
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
  }

  return (
    <OverviewResourceBarRoot>
      <AllResourceStatusSummary
        displayText="Resources"
        labelText="Status summary for all resources"
        resources={resources}
      />
      <GlobalNav {...globalNavProps} />
    </OverviewResourceBarRoot>
  )
}
