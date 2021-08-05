import {
  FilterLevel,
  filterSetFromLocation,
  FilterSource,
  isRegexp,
  TermState,
} from "./logfilters"

export type Tags = {
  [key: string]: string | undefined
  // action?: AnalyticsAction,
  type?: AnalyticsType
}

// The `type` tag describes what section of the UI
// that the analytics event takes place in
export enum AnalyticsType {
  Account = "account",
  Detail = "resource-detail",
  Grid = "grid",
  Shortcut = "shortcuts",
  Unknown = "unknown",
  Update = "update",
}

// The `action` tag describes the type of UI interaction
export enum AnalyticsAction {
  Click = "click",
  Collapse = "collapse",
  Edit = "edit",
  Expand = "expand",
  Load = "load",
  Shortcut = "shortcut",
  Star = "star",
  Unstar = "unstar",
  Push = "PUSH", // Reserved for location change events
}

// Fire and forget all analytics events
export const incr = (name: string, tags: Tags = {}): void => {
  let url = `//${window.location.host}/api/analytics`

  // Uncomment to debug analytics events
  // console.log("analytics \nname:", name, "\n payload:", tags)

  fetch(url, {
    method: "post",
    body: JSON.stringify([{ verb: "incr", name: name, tags: tags }]),
  })
}

export const pathToTag = (path: string): AnalyticsType => {
  if (path.indexOf("/") === 0) {
    path = path.substring(1) // chop off the leading /
  }
  let parts = path.split("/")
  if (parts[0] === "") {
    return AnalyticsType.Grid
  }
  if (parts[0] === "overview") {
    return AnalyticsType.Grid
  }

  if (parts[0] === "r") {
    if (parts[2] === "overview") {
      return AnalyticsType.Detail
    }
  }

  return AnalyticsType.Unknown
}

export let navigationToTags = (location: any, action: string): Tags => {
  let tags: Tags = { type: pathToTag(location.pathname) }
  if (action === "PUSH" && location.state?.action) {
    tags.action = location.state.action
  }

  let filterSet = filterSetFromLocation(location)
  if (filterSet.level != FilterLevel.all) {
    tags.level = filterSet.level
  }
  if (filterSet.source != FilterSource.all) {
    tags.source = filterSet.source
  }
  if (filterSet.term.state !== TermState.Empty) {
    const termType = isRegexp(filterSet.term.input) ? "regexp" : "text"
    tags.term = termType
  }
  return tags
}
