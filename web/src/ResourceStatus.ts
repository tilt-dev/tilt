import { ResourceStatus } from "./types"

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
