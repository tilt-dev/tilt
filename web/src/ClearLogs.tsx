import React from "react"
import styled from "styled-components"
import { Alert } from "./alerts"
import { FilterSource } from "./logfilters"
import { isBuildSpanId, isTiltfileSpanId } from "./logs"
import { useLogStore } from "./LogStore"
import { Color, FontSize } from "./style-helpers"

const ClearLogsButton = styled.a`
  margin-left: auto;
  cursor: pointer;
  font-size: ${FontSize.small};

  &:hover {
    color: ${Color.blue};
  }
`

export interface ClearLogsProps {
  resource?: Proto.webviewResource
  alerts?: Alert[]
}

const ClearLogs: React.FC<ClearLogsProps> = ({ resource, alerts }) => {
  const logStore = useLogStore()
  const label = resource?.name ? "Clear Logs" : "Clear All Logs"

  const clearLogs = () => {
    let spans: { [key: string]: Proto.webviewLogSpan }
    let relevantAlerts: Alert[]
    if (resource) {
      const manifestName = resource.name ?? ""
      spans = logStore.spansForManifest(manifestName)
      relevantAlerts = (alerts || []).filter(
        (alert) => alert.resourceName == manifestName
      )
    } else {
      spans = logStore.allSpans()
      relevantAlerts = alerts || []
    }

    // don't delete the latest span that corresponds to an alert
    for (const alert of relevantAlerts) {
      let spanIdsForResource = logStore.getOrderedSpansIdsForManifest(
        alert.resourceName
      )
      if (alert.source == FilterSource.build) {
        spanIdsForResource = spanIdsForResource.filter(
          (spanId) => isBuildSpanId(spanId) || isTiltfileSpanId(spanId)
        )
      } else if (alert.source == FilterSource.runtime) {
        spanIdsForResource = spanIdsForResource.filter(
          (spanId) => !isBuildSpanId(spanId) && !isTiltfileSpanId(spanId)
        )
      } else {
        // unsupported source, so don't attempt to prevent truncation
        continue
      }
      if (spanIdsForResource.length !== 0) {
        const latestSpanId = spanIdsForResource[spanIdsForResource.length - 1]
        delete spans[latestSpanId]
      }
    }

    logStore.removeSpans(Object.keys(spans))
  }

  return <ClearLogsButton onClick={() => clearLogs()}>{label}</ClearLogsButton>
}

export default ClearLogs
