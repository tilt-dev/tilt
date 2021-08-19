import React from "react"
import { Link } from "react-router-dom"
import styled from "styled-components"
import { ReactComponent as LogoWordmarkSvg } from "./assets/svg/logo-wordmark.svg"
import { CustomNav } from "./CustomNav"
import { GlobalNav } from "./GlobalNav"
import { usePathBuilder } from "./PathBuilder"
import {
  AllResourceStatusSummary,
  ResourceStatusSummaryRoot,
} from "./ResourceStatusSummary"
import { useSnapshotAction } from "./snapshot"
import { AnimDuration, Color, Font, FontSize, SizeUnit } from "./style-helpers"
import { TargetType } from "./types"
import { showUpdate } from "./UpdateDialog"

const HeaderBarRoot = styled.div`
  display: flex;
  align-items: center;
  padding-left: ${SizeUnit(1)};
  background-color: ${Color.grayDarkest};

  ${ResourceStatusSummaryRoot} {
    justify-self: center;
    flex-grow: 1;
    justify-content: center;
  }
`

const Logo = styled(LogoWordmarkSvg)`
  justify-self: flex-start;
  & .fillStd {
    transition: fill ${AnimDuration.short} ease;
    fill: ${Color.grayLightest};
  }
  &:hover .fillStd,
  &.isSelected .fillStd {
    fill: ${Color.gray7};
  }
  display: block;
`

const HeaderDivider = styled.div`
  border-left: 1px solid ${Color.grayLighter};
  height: ${SizeUnit(1)};
  margin: ${SizeUnit(0.5)};
`

const AllResourcesLink = styled(Link)`
  font-family: ${Font.monospace};
  color: ${Color.gray7};
  font-size: ${FontSize.small};
  text-decoration: none;
`

type HeaderBarProps = {
  view: Proto.webviewView
}

export default function HeaderBar(props: HeaderBarProps) {
  let isSnapshot = usePathBuilder().isSnapshot()
  let snapshot = useSnapshotAction()
  let view = props.view
  let session = view?.uiSession?.status
  let runningBuild = session?.runningTiltBuild
  let suggestedVersion = session?.suggestedTiltVersion
  let resources = view?.uiResources || []
  let hasK8s = resources.some((r) => {
    let specs = r.status?.specs ?? []
    return specs.some((spec) => spec.type === TargetType.K8s)
  })

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

  const pb = usePathBuilder()

  return (
    <HeaderBarRoot>
      <Link to="/overview">
        <Logo width="57px" />
      </Link>
      <HeaderDivider />
      <AllResourcesLink to={pb.encpath`/r/(all)/overview`}>
        All Resources
      </AllResourcesLink>
      <AllResourceStatusSummary resources={resources} />
      <CustomNav view={props.view} />
      <GlobalNav {...globalNavProps} />
    </HeaderBarRoot>
  )
}
