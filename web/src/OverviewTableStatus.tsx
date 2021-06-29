import React from "react"
import styled from "styled-components"
import { ReactComponent as CheckmarkSmallSvg } from "./assets/svg/checkmark-small.svg"
import { ReactComponent as CloseSvg } from "./assets/svg/close.svg"
import { ReactComponent as PendingSvg } from "./assets/svg/pending.svg"
import { Color, Glow, SizeUnit, spin } from "./style-helpers"
import { formatBuildDuration } from "./time"
import { ResourceStatus } from "./types"

const StyledOverviewTableStatus = styled.div``

const StatusIcon = styled.span`
  display: flex;
  align-items: center;
  margin-right: ${SizeUnit(0.2)};
  width: ${SizeUnit(0.5)};

  svg {
    width: 100%;
  }
`
const StatusMsg = styled.span``
const StatusLine = styled.div`
  display: flex;
  align-items: center;

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

type OverviewTableStatusProps = {
  status: ResourceStatus
  lastBuildDur?: moment.Duration | null
  alertCount: number
  isBuild?: boolean
}

export default function OverviewTableStatus(props: OverviewTableStatusProps) {
  let { status, lastBuildDur, alertCount, isBuild } = props
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

  return (
    <StyledOverviewTableStatus>
      {msg && (
        <StatusLine className={classes}>
          <StatusIcon>{icon}</StatusIcon>
          <StatusMsg>{msg}</StatusMsg>
        </StatusLine>
      )}
    </StyledOverviewTableStatus>
  )
}
