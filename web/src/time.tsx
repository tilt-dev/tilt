import moment from "moment"

export const zeroTime = "0001-01-01T00:00:00Z"

export function isZeroTime(time: string) {
  return !time || time === zeroTime
}

// Duplicated internal/hud/format.go
export function formatBuildDuration(d: moment.Duration): string {
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

export function timeDiff(start: string, end: string): moment.Duration {
  let t1 = moment(start || zeroTime)
  let t2 = moment(end || zeroTime)
  return moment.duration(t2.diff(t1))
}
