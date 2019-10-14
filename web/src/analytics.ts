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
  if (path.startsWith("/r/") && !path.endsWith("/alerts")) {
    return "log"
  } else if (path.endsWith("/alerts")) {
    return "errors"
  } else if (path === "/") {
    return "all"
  }

  return "unknown"
}

export { incr, pathToTag }
