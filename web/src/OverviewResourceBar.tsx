import React from "react"
import styled from "styled-components"
import { GlobalNav, GlobalNavRoot } from "./GlobalNav"
import { usePathBuilder } from "./PathBuilder"
import { ResourceStatusSummary } from "./ResourceStatusSummary"
import { useSnapshotAction } from "./snapshot"
import { SizeUnit } from "./style-helpers"
import { TargetType } from "./types"
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
  let runningBuild = view?.runningTiltBuild
  let suggestedVersion = view?.suggestedTiltVersion
  let resources = view?.resources || []
  let hasK8s = resources.some((r) => {
    let specs = r.specs ?? []
    return specs.some((spec) => spec.type === TargetType.K8s)
  })

  let globalNavProps = {
    isSnapshot,
    snapshot,
    showUpdate: showUpdate(view),
    suggestedVersion,
    runningBuild,
    tiltCloudUsername: view.tiltCloudUsername ?? "",
    tiltCloudSchemeHost: view.tiltCloudSchemeHost ?? "",
    tiltCloudTeamID: view.tiltCloudTeamID ?? "",
    tiltCloudTeamName: view.tiltCloudTeamName ?? "",
  }

  return (
    <OverviewResourceBarRoot>
      <ResourceStatusSummary view={props.view} />
      <GlobalNav {...globalNavProps} />
    </OverviewResourceBarRoot>
  )
}
