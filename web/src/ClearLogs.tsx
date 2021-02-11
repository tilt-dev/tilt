import React from "react"
import styled from "styled-components"
import { incr } from "./analytics"
import LogStore, { useLogStore } from "./LogStore"
import {
  AnimDuration,
  Color,
  FontSize,
  mixinResetButtonStyle,
} from "./style-helpers"
import { ResourceName } from "./types"

const ClearLogsButton = styled.button`
  ${mixinResetButtonStyle}
  margin-left: auto;
  font-size: ${FontSize.small};
  color: ${Color.white};
  transition: color ${AnimDuration.default} ease;

  &:hover {
    color: ${Color.blue};
  }
`

export interface ClearLogsProps {
  resourceName: string
}

export const clearLogs = (
  logStore: LogStore,
  resourceName: string,
  action: string
) => {
  let spans: { [key: string]: Proto.webviewLogSpan }
  const all = resourceName === ResourceName.all
  if (all) {
    spans = logStore.allSpans()
  } else {
    spans = logStore.spansForManifest(resourceName)
  }
  incr("ui.web.clearLogs", { action, all: all.toString() })
  logStore.removeSpans(Object.keys(spans))
}

const ClearLogs: React.FC<ClearLogsProps> = ({ resourceName }) => {
  const logStore = useLogStore()
  const label =
    resourceName == ResourceName.all ? "Clear All Logs" : "Clear Logs"

  return (
    <ClearLogsButton onClick={() => clearLogs(logStore, resourceName, "click")}>
      {label}
    </ClearLogsButton>
  )
}

export default ClearLogs
