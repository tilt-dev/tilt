// Helper functions for dealing with logs

import { LogLine, ResourceName } from "./types"

export function logLinesFromString(
  log: string,
  manifestName?: string
): LogLine[] {
  let lines = log.split("\n")
  return lines.map((text) => {
    return {
      text: text,
      manifestName: manifestName ?? "",
      level: "INFO",
      spanId: "",
      storedLineIndex: 0,
    }
  })
}

export function logLinesToString(
  lines: LogLine[],
  showManifestPrefix: boolean
): string {
  return lines
    .map((line) => {
      let text = line.text
      if (showManifestPrefix) {
        text = sourcePrefix(line.manifestName) + text
      }
      return text
    })
    .join("\n")
}

export function sourcePrefix(n: string) {
  if (n === "" || n === ResourceName.tiltfile) {
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

export function isBuildSpanId(spanId: string): boolean {
  return spanId.indexOf("build:") === 0 || spanId.indexOf("cmdimage:") === 0
}
