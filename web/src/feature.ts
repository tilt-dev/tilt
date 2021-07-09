// This Features wrapper behaves differently than the one in Go.
// Checking for features that don't exist is *not* an error here.
// This is important because when the React app starts,
// it starts with an empty state and there won't be _any_ feature flags
// until the first engine state comes in over the Websocket.
import { createContext, useContext } from "react"

type featureFlags = { [featureFlag in Flag]?: boolean }

// Flag names are defined in internal/feature/flags.go
export enum Flag {
  MultipleContainersPerPod = "multiple_containers_per_pod",
  Events = "events",
  Snapshots = "snapshots",
  UpdateHistory = "update_history",
  Facets = "facets",
  Labels = "labels",
}

export default class Features {
  private flags: featureFlags

  constructor(flags: object | null | undefined) {
    if (flags) {
      this.flags = flags as featureFlags
    } else {
      this.flags = {}
    }
  }

  public isEnabled(flag: Flag): boolean {
    if (this.flags.hasOwnProperty(flag)) {
      return this.flags[flag] as boolean
    }
    return false
  }
}

export const FeaturesContext = createContext<Features>(new Features({}))
FeaturesContext.displayName = "Features"

export function useFeatures(): Features {
  return useContext(FeaturesContext)
}

export const FeaturesProvider = FeaturesContext.Provider
