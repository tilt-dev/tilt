import { Font } from "./style-helpers"
import {
  ResourceDisableState,
  ResourceStatus,
  TargetType,
  UIResource,
} from "./types"

export const disabledResourceStyleMixin = `
font-family: ${Font.sansSerif};
font-style: italic;
font-size: 14px; /* Use non-standard font-size, since sans-serif font looks larger than monospace font */
`

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
    case ResourceStatus.Disabled:
      return "isDisabled"
    case ResourceStatus.None:
      return "isNone"
  }
}

export function resourceIsDisabled(resource: UIResource | undefined): boolean {
  if (!resource) {
    return false
  }

  // Consider both "pending" and "disabled" states as disabled resources
  const disableState = resource.status?.disableStatus?.state
  if (
    disableState === ResourceDisableState.Pending ||
    disableState === ResourceDisableState.Disabled
  ) {
    return true
  }

  return false
}

// Choose the best identifier for the type of this resource.
// The deploy type (k8s, dc) is always preferred.
export function resourceTargetType(resource: UIResource): string {
  let specs = resource.status?.specs || []
  let result = TargetType.Unspecified as string
  specs.forEach((spec) => {
    if (spec.type == "" || spec.type == TargetType.Unspecified) {
      return
    }
    if (spec.type == TargetType.Image) {
      if (result == TargetType.Unspecified) {
        result = spec.type
      }
      return
    }
    if (spec.type) {
      result = spec.type
    }
  })
  return result
}
