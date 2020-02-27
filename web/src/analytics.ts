type Tags = { [key: string]: string }

// Fire and forget all analytics events
const incr = (name: string, tags: Tags = {}): void => {
  let url = `//${window.location.host}/api/analytics`

  fetch(url, {
    method: "post",
    body: JSON.stringify([{ verb: "incr", name: name, tags: tags }]),
  })
}

const pathToTag = (path: string): string => {
  if (path.indexOf("/") == 0) {
    path = path.substring(1) // chop off the leading /
  }
  let parts = path.split("/")
  if (parts[0] == "") {
    return "all"
  }
  if (parts[0] == "alerts") {
    return "errors"
  }
  if (parts[0] == "facets") {
    return "facets"
  }
  if (parts[0] == "trace") {
    return "trace"
  }

  if (parts[0] == "r") {
    if (parts.length <= 2) {
      return "log"
    }
    if (parts[2] == "alerts" || parts[2] == "errors") {
      return "errors"
    }
    if (parts[2] == "facets") {
      return "facets"
    }
    if (parts[2] == "trace") {
      return "trace"
    }
  }

  return "unknown"
}

export { incr, pathToTag }
