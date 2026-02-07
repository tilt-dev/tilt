import React from "react"
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

export const copyLogs = (logStore: LogStore, resourceName: string) => {
  const all = resourceName === ResourceName.all
  const lines = all ? logStore.allLog() : logStore.manifestLog(resourceName)
  const text = logLinesToString(lines, !all)
  navigator.clipboard.writeText(text)
}

const CopyLogs: React.FC<CopyLogsProps> = ({ resourceName }) => {
  const logStore = useLogStore()
  const all = resourceName == ResourceName.all
  const label = all ? "Copy All Logs" : "Copy Logs"

  return (
    <CopyLogsButton onClick={() => copyLogs(logStore, resourceName)}>
      {label}
    </CopyLogsButton>
  )
}

export default CopyLogs
