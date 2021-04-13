import React from "react"
import styled from "styled-components"
import { InstrumentedButton } from "./instrumentedComponents"
import LogStore, { useLogStore } from "./LogStore"
import {
  AnimDuration,
  Color,
  FontSize,
  mixinResetButtonStyle,
} from "./style-helpers"
import { ResourceName } from "./types"

const ClearLogsButton = styled(InstrumentedButton)`
  ${mixinResetButtonStyle};
  margin-left: 1rem;
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

export const clearLogs = (logStore: LogStore, resourceName: string) => {
  let spans: { [key: string]: Proto.webviewLogSpan }
  const all = resourceName === ResourceName.all
  if (all) {
    spans = logStore.allSpans()
  } else {
    spans = logStore.spansForManifest(resourceName)
  }
  logStore.removeSpans(Object.keys(spans))
}

const ClearLogs: React.FC<ClearLogsProps> = ({ resourceName }) => {
  const logStore = useLogStore()
  const all = resourceName == ResourceName.all
  const label = all ? "Clear All Logs" : "Clear Logs"

  return (
    <ClearLogsButton
      onClick={() => clearLogs(logStore, resourceName)}
      analyticsName="ui.web.clearLogs"
      analyticsTags={{ all: all.toString() }}
    >
      {label}
    </ClearLogsButton>
  )
}

export default ClearLogs
