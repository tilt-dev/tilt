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

// Matches ANSI escape sequences (colors, cursor movement, etc.)
// eslint-disable-next-line no-control-regex
const ansiRegex = /\x1b\[[0-9;]*[a-zA-Z]|\x1b\].*?\x07/g

export function stripAnsiCodes(text: string): string {
  return text.replace(ansiRegex, "")
}

export function logLinesToString(
  lines: LogLine[],
  showManifestPrefix: boolean
): string {
  return lines
    .map((line) => {
      let text = stripAnsiCodes(line.text)
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
