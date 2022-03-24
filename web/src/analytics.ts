import { Action, Location } from "history"
import {
  FilterLevel,
  filterSetFromLocation,
  FilterSource,
  isRegexp,
  TermState,
} from "./logfilters"
import PathBuilder from "./PathBuilder"

export const emptyTags = Object.freeze({})

export type Tags = {
  [key: string]: string | undefined
  action?: AnalyticsAction | Action
  type?: AnalyticsType
}

// The `type` tag describes what section of the UI
// that the analytics event takes place in
export enum AnalyticsType {
  Account = "account",
  Detail = "resource-detail",
  Grid = "grid", // aka Table View
  Shortcut = "shortcuts",
  Unknown = "unknown",
  Update = "update",
}

// The `action` tag describes the type of UI interaction
export enum AnalyticsAction {
  Click = "click",
  Close = "close",
  Collapse = "collapse",
  Edit = "edit",
  Expand = "expand",
  Load = "load",
  Shortcut = "shortcut",
  Star = "star",
  Unstar = "unstar",
}

// Fire and forget all analytics events
export const incr = (pb: PathBuilder, name: string, tags: Tags = {}): void => {
  if (pb.isSnapshot()) {
    return
  }

  let url = pb.path("/api/analytics")

  // Uncomment to debug analytics events
  // console.log("analytics event: \nname:", name, "\npayload:", tags)

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

export let navigationToTags = (
  location: Location,
  action: AnalyticsAction | Action
): Tags => {
  let tags: Tags = { type: pathToTag(location.pathname) }

  // If the location has a `state`, use the `action` property for the analytics event
  const locationAction: Action | undefined = location.state
    ? (location as Location<{ action?: Action }>).state?.action
    : undefined
  if (action === "PUSH" && locationAction) {
    tags.action = locationAction
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
