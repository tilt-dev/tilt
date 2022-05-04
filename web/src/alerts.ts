import { FilterLevel, FilterSource } from "./logfilters"
import { LogAlert, LogAlertIndex } from "./LogStore"
import { LogLevel, UIResource } from "./types"

export type Alert = {
  source: FilterSource
  level: FilterLevel
  resourceName: string
}

// Runtime alerts can only come from the current pod.
//
// All alerts are derived from the log store, to ensure that clearing logs
// also clears the alerts.
//
// TODO(nick): Add support for docker compose, local resource runtime alerts.
function runtimeAlerts(r: UIResource, alertIndex: LogAlertIndex): Alert[] {
  let name = r.metadata?.name || ""
  let spanId = r.status?.k8sResourceInfo?.spanID || ""
  if (!spanId) {
    return []
  }
  return alertIndex.alertsForSpanId(spanId).map((logAlert: LogAlert): Alert => {
    return {
      resourceName: name,
      source: FilterSource.runtime,
      level:
        logAlert.level == LogLevel.WARN ? FilterLevel.warn : FilterLevel.error,
    }
  })
}

// Build alerts can only come from the most recently finished build.
//
// All alerts are derived from the log store, to ensure that clearing logs
// also clears the alerts.
function buildAlerts(r: UIResource, alertIndex: LogAlertIndex): Alert[] {
  let name = r.metadata?.name || ""
  const latestBuild = (r.status?.buildHistory ?? [])[0]
  const spanId = latestBuild?.spanID ?? ""
  if (!spanId) {
    return []
  }
  return alertIndex.alertsForSpanId(spanId).map((logAlert: LogAlert): Alert => {
    return {
      resourceName: name,
      source: FilterSource.build,
      level:
        logAlert.level == LogLevel.WARN ? FilterLevel.warn : FilterLevel.error,
    }
  })
}

function combinedAlerts(r: UIResource, alertIndex: LogAlertIndex): Alert[] {
  let result = runtimeAlerts(r, alertIndex)
  result.push(...buildAlerts(r, alertIndex))
  return result
}

function buildWarningCount(r: UIResource, alertIndex: LogAlertIndex): number {
  return buildAlerts(r, alertIndex).filter((alert) => {
    return alert.level == FilterLevel.warn
  }).length
}

function runtimeWarningCount(r: UIResource, alertIndex: LogAlertIndex): number {
  return runtimeAlerts(r, alertIndex).filter((alert) => {
    return alert.level == FilterLevel.warn
  }).length
}

export {
  combinedAlerts,
  buildAlerts,
  runtimeAlerts,
  buildWarningCount,
  runtimeWarningCount,
}
