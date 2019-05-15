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
  if (
    path.startsWith("/r/") &&
    !path.endsWith("/preview") &&
    !path.endsWith("/errors")
  ) {
    return "log"
  } else if (path.endsWith("/preview")) {
    return "preview"
  } else if (path.endsWith("/errors")) {
    return "errors"
  } else if (path === "/") {
    return "all"
  }

  return "unknown"
}

export { incr, pathToTag }
