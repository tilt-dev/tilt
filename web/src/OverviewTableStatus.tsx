import React from "react"
import styled from "styled-components"
import { ReactComponent as CheckmarkSmallSvg } from "./assets/svg/checkmark-small.svg"
import { ReactComponent as CloseSvg } from "./assets/svg/close.svg"
import { ReactComponent as PendingSvg } from "./assets/svg/pending.svg"
import { useResourceNav } from "./ResourceNav"
import {
  Color,
  Glow,
  mixinResetButtonStyle,
  SizeUnit,
  spin,
} from "./style-helpers"
import { formatBuildDuration } from "./time"
import { ResourceStatus } from "./types"

const StatusMsg = styled.span``
const StyledOverviewTableStatus = styled.button`
  ${mixinResetButtonStyle};
  color: inherit;
  display: flex;
  align-items: center;

  & + & {
    margin-top: ${SizeUnit(0.15)};
  }

  &:hover ${StatusMsg} {
    text-decoration: underline;
    text-underline-position: under;
  }
  &.is-healthy {
    svg {
      fill: ${Color.green};
    }
  }
  &.is-building,
  &.is-pending {
    svg {
      fill: ${Color.grayLightest};
      animation: ${spin} 4s linear infinite;
      width: 80%;
    }
  }
  &.is-pending {
    ${StatusMsg} {
      animation: ${Glow.opacity} 2s linear infinite;
    }
  }
  &.is-error {
    color: ${Color.red};
    svg {
      fill: ${Color.red};
    }
  }
  &.is-none {
    color: ${Color.grayLight};
  }
`
const StatusIcon = styled.span`
  display: flex;
  align-items: center;
  margin-right: ${SizeUnit(0.2)};
  width: ${SizeUnit(0.5)};

  svg {
    width: 100%;
  }
`

type OverviewTableStatusProps = {
  status: ResourceStatus
  resourceName: string
  lastBuildDur?: moment.Duration | null
  alertCount: number
  isBuild?: boolean
}

export default function OverviewTableStatus(props: OverviewTableStatusProps) {
  let { status, lastBuildDur, alertCount, isBuild, resourceName } = props
  let icon = null
  let msg = ""
  let classes = ""

  switch (status) {
    case ResourceStatus.Building:
      icon = <PendingSvg />
      msg = isBuild ? "Updating…" : "Runtime Deploying"
      classes = "is-building"
      break
    case ResourceStatus.None:
      break
    case ResourceStatus.Pending:
      icon = <PendingSvg />
      msg = isBuild ? "Update Pending" : "Runtime Pending"
      classes = "is-pending"
      break
    case ResourceStatus.Warning:
      break
    case ResourceStatus.Healthy:
      let buildDurText = lastBuildDur
        ? ` in ${formatBuildDuration(lastBuildDur)}`
        : ""
      icon = <CheckmarkSmallSvg />
      msg = isBuild ? `Updated${buildDurText}` : "Runtime Ready"
      classes = "is-healthy"

      if (alertCount > 0) {
        msg += ", with issues"
      }
      break
    case ResourceStatus.Unhealthy:
      icon = <CloseSvg />
      msg = isBuild ? "Update error" : "Runtime error"
      classes = "is-error"
      break
    default:
      msg = ""
  }

  let nav = useResourceNav()

  if (!msg) return null

  return (
    <StyledOverviewTableStatus
      className={classes}
      onClick={() => void nav.openResource(resourceName)}
    >
      <StatusIcon>{icon}</StatusIcon>
      <StatusMsg>{msg}</StatusMsg>
    </StyledOverviewTableStatus>
  )
}
