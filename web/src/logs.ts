// Helper functions for dealing with logs

import { LogLine } from "./types"

export function logLinesFromString(
  log: string,
  manifestName?: string
): LogLine[] {
  let lines = log.split("\n")
  return lines.map(text => {
    return {
      text: text,
      manifestName: manifestName ?? "",
      level: "INFO",
    }
  })
}

export function logLinesToString(
  lines: LogLine[],
  showManifestPrefix: boolean
): string {
  return lines
    .map(line => {
      let text = line.text
      if (showManifestPrefix) {
        text = sourcePrefix(line.manifestName) + text
      }
      return text
    })
    .join("\n")
}

export function sourcePrefix(n: string) {
  if (n == "" || n == "(Tiltfile)") {
    return ""
  }
  let max = 12
  let spaces = ""
  if (n.length > max) {
    n = n.substring(0, max - 1) + "…"
  } else {
    spaces = " ".repeat(max - n.length)
  }
  return n + spaces + "┊ "
}
