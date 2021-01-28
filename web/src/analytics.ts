import { FilterLevel, filterSetFromLocation, FilterSource } from "./logfilters"

export type Tags = { [key: string]: string }

// Fire and forget all analytics events
export const incr = (name: string, tags: Tags = {}): void => {
  let url = `//${window.location.host}/api/analytics`

  fetch(url, {
    method: "post",
    body: JSON.stringify([{ verb: "incr", name: name, tags: tags }]),
  })
}

export const pathToTag = (path: string): string => {
  if (path.indexOf("/") === 0) {
    path = path.substring(1) // chop off the leading /
  }
  let parts = path.split("/")
  if (parts[0] === "") {
    return "all"
  }
  if (parts[0] === "alerts") {
    return "errors"
  }
  if (parts[0] === "facets") {
    return "facets"
  }
  if (parts[0] === "trace") {
    return "trace"
  }
  if (parts[0] === "metrics") {
    return "metrics"
  }
  if (parts[0] === "overview") {
    return "grid"
  }

  if (parts[0] === "r") {
    if (parts.length <= 2) {
      return "log"
    }
    if (parts[2] === "alerts" || parts[2] === "errors") {
      return "errors"
    }
    if (parts[2] === "facets") {
      return "facets"
    }
    if (parts[2] === "trace") {
      return "trace"
    }
    if (parts[2] === "metrics") {
      return "metrics"
    }
    if (parts[2] === "overview") {
      return "resource-detail"
    }
  }

  return "unknown"
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
  return tags
}
