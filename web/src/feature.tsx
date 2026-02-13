// This Features wrapper behaves differently than the one in Go.
// Checking for features that don't exist is *not* an error here.
// This is important because when the React app starts,
// it starts with an empty state and there won't be _any_ feature flags
// until the first engine state comes in over the Websocket.
import { createContext, PropsWithChildren, useContext, useMemo } from "react"
import type { UIFeatureFlag } from "./core"

type featureFlags = { [featureFlag in Flag]?: boolean }

// Flag names are defined in internal/feature/flags.go
export enum Flag {
  Events = "events",
  Snapshots = "snapshots",
  Labels = "labels",
}

export default class Features {
  private readonly flags: featureFlags

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

// Server-side flags are formatted as a list.
// Many tests uses the {key: value} format.
export function FeaturesProvider(
  props: PropsWithChildren<{
    featureFlags: UIFeatureFlag[] | null
  }>
) {
  let flagList = props.featureFlags || []
  let features = useMemo(() => {
    let featureFlags = {} as { [key: string]: boolean }
    flagList.forEach((flag) => {
      featureFlags[flag.name || ""] = !!flag.value
    })
    return new Features(featureFlags)
  }, [flagList])

  return (
    <FeaturesContext.Provider value={features}>
      {props.children}
    </FeaturesContext.Provider>
  )
}

export let FeaturesTestProvider = FeaturesContext.Provider
