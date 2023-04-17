// Helper functions for displaying links

// If a URL contains the hostname 0.0.0.0, resolve that to the current
// window.location.hostname. This ensures that tilt sends users to the
// right place when it's being accessed from a remote machine.
export function resolveURL(input: string): string {
  if (!input) {
    return input
  }
  try {
    let url = new URL(input)
    if (url.hostname === "0.0.0.0") {
      url.hostname = window.location.hostname
      return url.toString()
    }
  } catch (err) {} // fall through
  return input
}

export function displayURL(url: string): string {
  url = url.replace(/^http:\/\//, "")
  url = url.replace(/^https:\/\//, "")
  url = url.replace(/^www\./, "")
  url = url.replace(/^([^\/]+)\/$/, "$1")
  return url || ""
}
