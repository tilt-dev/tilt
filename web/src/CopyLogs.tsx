import React, { useState } from "react"
import styled from "styled-components"
import { InstrumentedButton } from "./instrumentedComponents"
import { FilterSet } from "./logfilters"
import LogStore, { useLogStore } from "./LogStore"
import { filterLogLinesForDisplay, logLinesToString } from "./logs"
import { useStarredResources } from "./StarredResourcesContext"
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
  font-size: ${FontSize.small};
  color: ${Color.white};
  transition: color ${AnimDuration.default} ease;

  &:hover {
    color: ${Color.blue};
  }
`

export interface CopyLogsProps {
  resourceName: string
  filterSet: FilterSet
}

function logLinesForResource(
  logStore: LogStore,
  resourceName: string,
  starredResources: string[]
) {
  const all = resourceName === ResourceName.all
  if (all) {
    return logStore.allLog()
  }
  if (resourceName === ResourceName.starred) {
    return logStore.starredLogPatchSet(starredResources, 0).lines
  }
  return logStore.manifestLog(resourceName)
}

export const copyLogs = (
  logStore: LogStore,
  resourceName: string,
  filterSet: FilterSet,
  starredResources: string[] = []
): number => {
  const all = resourceName === ResourceName.all
  const lines = logLinesForResource(logStore, resourceName, starredResources)
  const visibleLines = filterLogLinesForDisplay(lines, filterSet)
  const text = logLinesToString(visibleLines, !all)
  navigator.clipboard.writeText(text)
  return visibleLines.length
}

const CopyLogs: React.FC<CopyLogsProps> = ({ resourceName, filterSet }) => {
  const logStore = useLogStore()
  const { starredResources } = useStarredResources()
  const label = "Copy"
  const [tooltipOpen, setTooltipOpen] = useState(false)
  const [tooltipText, setTooltipText] = useState("")

  const handleClick = () => {
    const lineCount = copyLogs(
      logStore,
      resourceName,
      filterSet,
      starredResources
    )
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
      <CopyLogsButton onClick={handleClick}>{label}</CopyLogsButton>
    </TiltTooltip>
  )
}

export default CopyLogs
