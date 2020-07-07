import moment from "moment"

// Duplicated internal/hud/format.go
function formatBuildDuration(d: moment.Duration): string {
  let hours = Math.floor(d.asHours())
  if (hours > 0) {
    return `${hours}h`
  }

  let minutes = Math.floor(d.asMinutes())
  if (minutes > 0) {
    return `${minutes}m`
  }

  let seconds = d.asSeconds()
  if (seconds >= 9.95) {
    return `${Math.round(seconds)}s`
  }

  return `${seconds.toFixed(1)}s`
}

export { formatBuildDuration }
