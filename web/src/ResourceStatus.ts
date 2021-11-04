import { ResourceStatus, UIResource } from "./types"

export function ClassNameFromResourceStatus(rs: ResourceStatus): string {
  switch (rs) {
    case ResourceStatus.Building:
      return "isBuilding"
    case ResourceStatus.Pending:
      return "isPending"
    case ResourceStatus.Warning:
      return "isWarning"
    case ResourceStatus.Healthy:
      return "isHealthy"
    case ResourceStatus.Unhealthy:
      return "isUnhealthy"
    case ResourceStatus.None:
      return "isNone"
  }
}

export function resourceIsDisabled(resource: UIResource): boolean {
  const disableCount = resource.status?.disableStatus?.disabledCount ?? 0
  if (disableCount > 0) {
    return true
  }

  return false
}
