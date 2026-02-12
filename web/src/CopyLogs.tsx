import React, { useState } from "react"
import styled from "styled-components"
import { InstrumentedButton } from "./instrumentedComponents"
import LogStore, { useLogStore } from "./LogStore"
import { logLinesToString } from "./logs"
import {
  AnimDuration,
  Color,
  FontSize,
  mixinResetButtonStyle,
} from "./style-helpers"
import TiltTooltip from "./Tooltip"
import { ResourceName } from "./types"

const CopyLogsButton = styled(InstrumentedButton)`
  ${mixinResetButtonStyle};
  margin-left: 1rem;
  font-size: ${FontSize.small};
  color: ${Color.white};
  transition: color ${AnimDuration.default} ease;

  &:hover {
    color: ${Color.blue};
  }
`

export interface CopyLogsProps {
  resourceName: string
}

export const copyLogs = (
  logStore: LogStore,
  resourceName: string
): number => {
  const all = resourceName === ResourceName.all
  const lines = all ? logStore.allLog() : logStore.manifestLog(resourceName)
  const text = logLinesToString(lines, !all)
  navigator.clipboard.writeText(text)
  return lines.length
}

const CopyLogs: React.FC<CopyLogsProps> = ({ resourceName }) => {
  const logStore = useLogStore()
  const all = resourceName == ResourceName.all
  const label = all ? "Copy All Logs" : "Copy Logs"
  const [tooltipOpen, setTooltipOpen] = useState(false)
  const [tooltipText, setTooltipText] = useState("")

  const handleClick = () => {
    const lineCount = copyLogs(logStore, resourceName)
    setTooltipText(
      lineCount === 1 ? "Copied 1 line" : `Copied ${lineCount} lines`
    )
    setTooltipOpen(true)
    setTimeout(() => setTooltipOpen(false), 1500)
  }

  return (
    <TiltTooltip
      title={tooltipText}
      open={tooltipOpen}
      disableHoverListener
      disableFocusListener
      placement="top"
    >
      <span>
        <CopyLogsButton onClick={handleClick}>{label}</CopyLogsButton>
      </span>
    </TiltTooltip>
  )
}

export default CopyLogs
