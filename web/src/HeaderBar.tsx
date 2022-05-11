import React from "react"
import { Link } from "react-router-dom"
import styled from "styled-components"
import { AnalyticsType } from "./analytics"
import { ReactComponent as DetailViewSvg } from "./assets/svg/detail-view-icon.svg"
import { ReactComponent as LogoWordmarkSvg } from "./assets/svg/logo-wordmark.svg"
import { ReactComponent as TableViewSvg } from "./assets/svg/table-view-icon.svg"
import { CustomNav } from "./CustomNav"
import { GlobalNav, GlobalNavProps } from "./GlobalNav"
import { usePathBuilder } from "./PathBuilder"
import {
  AllResourceStatusSummary,
  ResourceStatusSummaryRoot,
} from "./ResourceStatusSummary"
import { useSnapshotAction } from "./snapshot"
import { SnapshotBar } from "./SnapshotBar"
import { AnimDuration, Color, Font, FontSize, SizeUnit } from "./style-helpers"
import { showUpdate } from "./UpdateDialog"

const HeaderBarRoot = styled.nav`
  display: flex;
  align-items: center;
  padding-left: ${SizeUnit(1)};
  background-color: ${Color.gray10};

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
    fill: ${Color.gray70};
  }
  display: block;
`

const HeaderDivider = styled.div`
  border-left: 1px solid ${Color.gray40};
  height: ${SizeUnit(0.7)};
  margin: ${SizeUnit(0.5)};
`

const ViewLinkText = styled.span`
  bottom: 0;
  color: ${Color.gray70};
  font-family: ${Font.monospace};
  font-size: ${FontSize.smallest};
  opacity: 0;
  position: absolute;
  transition: opacity ${AnimDuration.default} ease;
  white-space: nowrap;
  width: 100%;
`

const viewLinkIconMixin = `
  display: flex;
  transition: fill ${AnimDuration.default} ease;
  height: 100%;
  padding: ${SizeUnit(0.65)} 0;
  fill: ${Color.gray50};

  &.isCurrent {
    fill: ${Color.gray70};
  }
`

const TableViewIcon = styled(TableViewSvg)`
  ${viewLinkIconMixin}
  /* "Hack" to right-align text */
  padding-left: ${SizeUnit(0.5)};
`

const DetailViewIcon = styled(DetailViewSvg)`
  ${viewLinkIconMixin}
`

const ViewLink = styled(Link)`
  position: relative;

  &:is(:hover, :focus, :active) {
    ${ViewLinkText} {
      opacity: 1;
    }

    ${TableViewIcon}, ${DetailViewIcon} {
      fill: ${Color.blue};
    }
  }
`

const ViewLinkSection = styled.div`
  align-items: center;
  display: flex;
  margin-left: ${SizeUnit(1)};
  margin-right: ${SizeUnit(1)};
`

type HeaderBarProps = {
  view: Proto.webviewView
  isSocketConnected: boolean
  currentPage?: AnalyticsType.Detail | AnalyticsType.Grid
}

export default function HeaderBar({
  view,
  currentPage,
  isSocketConnected,
}: HeaderBarProps) {
  let isSnapshot = usePathBuilder().isSnapshot()
  let snapshot = useSnapshotAction()
  let session = view?.uiSession?.status
  let runningBuild = session?.runningTiltBuild
  let suggestedVersion = session?.suggestedTiltVersion
  let resources = view?.uiResources || []

  let globalNavProps: GlobalNavProps = {
    isSnapshot,
    snapshot,
    showUpdate: showUpdate(view),
    suggestedVersion,
    runningBuild,
    tiltCloudUsername: session?.tiltCloudUsername ?? "",
    tiltCloudSchemeHost: session?.tiltCloudSchemeHost ?? "",
    tiltCloudTeamID: session?.tiltCloudTeamID ?? "",
    tiltCloudTeamName: session?.tiltCloudTeamName ?? "",
    clusterConnections: view.clusters,
  }

  const pb = usePathBuilder()

  const tableViewLinkClass =
    currentPage === AnalyticsType.Grid ? "isCurrent" : ""
  const detailViewLinkClass =
    currentPage === AnalyticsType.Detail ? "isCurrent" : ""

  // TODO (lizz): Consider refactoring nav to use more semantic pattern of ul + li
  return (
    <>
      <SnapshotBar className={`is-${currentPage}`} />
      <HeaderBarRoot aria-label="Dashboard menu">
        <Link to="/overview" aria-label="Tilt home">
          <Logo width="57px" />
        </Link>
        <ViewLinkSection>
          <ViewLink
            to="/overview"
            aria-label="Table view"
            aria-current={currentPage === AnalyticsType.Grid}
          >
            <TableViewIcon className={tableViewLinkClass} role="presentation" />
            <ViewLinkText>Table</ViewLinkText>
          </ViewLink>
          <HeaderDivider role="presentation" />
          <ViewLink
            to={pb.encpath`/r/(all)/overview`}
            aria-label="Detail view"
            aria-current={currentPage === AnalyticsType.Detail}
          >
            <DetailViewIcon
              className={detailViewLinkClass}
              role="presentation"
            />
            <ViewLinkText>Detail</ViewLinkText>
          </ViewLink>
        </ViewLinkSection>
        <AllResourceStatusSummary
          displayText="Resources"
          labelText="Status summary for all resources"
          resources={resources}
          isSocketConnected={isSocketConnected}
        />
        <CustomNav view={view} />
        <GlobalNav {...globalNavProps} />
      </HeaderBarRoot>
    </>
  )
}
