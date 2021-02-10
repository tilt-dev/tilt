import React from "react"
import styled from "styled-components"
import { useLogStore } from "./LogStore"
import {
  AnimDuration,
  Color,
  FontSize,
  mixinResetButtonStyle,
} from "./style-helpers"

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
  resource?: Proto.webviewResource
}

const ClearLogs: React.FC<ClearLogsProps> = ({ resource }) => {
  const logStore = useLogStore()
  const label = resource?.name ? "Clear Logs" : "Clear All Logs"

  const clearLogs = () => {
    let spans: { [key: string]: Proto.webviewLogSpan }
    if (resource) {
      const manifestName = resource.name ?? ""
      spans = logStore.spansForManifest(manifestName)
    } else {
      spans = logStore.allSpans()
    }

    logStore.removeSpans(Object.keys(spans))
  }

  return <ClearLogsButton onClick={() => clearLogs()}>{label}</ClearLogsButton>
}

export default ClearLogs
