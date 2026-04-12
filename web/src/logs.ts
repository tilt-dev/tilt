// Helper functions for dealing with logs

import {
  FilterLevel,
  FilterSet,
  FilterSource,
  TermState,
} from "./logfilters"
import { LogLine, ResourceName } from "./types"

export const DISPLAY_LOG_PROLOGUE_LENGTH = 5

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

export function logLineMatchesTermFilter(
  line: LogLine,
  filterSet: FilterSet
): boolean {
  const { term } = filterSet

  if (!term || term.state !== TermState.Parsed) {
    return true
  }

  return term.regexp.test(line.text)
}

export function logLineMatchesLevelFilter(
  line: LogLine,
  filterSet: FilterSet
): boolean {
  let level = filterSet.level
  if (level === FilterLevel.warn && line.level !== "WARN") {
    return false
  }
  if (level === FilterLevel.error && line.level !== "ERROR") {
    return false
  }
  return true
}

export function logLineMatchesDisplayFilter(
  line: LogLine,
  filterSet: FilterSet
): boolean {
  if (line.buildEvent) {
    return true
  }

  let source = filterSet.source
  if (source === FilterSource.runtime && isBuildSpanId(line.spanId)) {
    return false
  }
  if (source === FilterSource.build && !isBuildSpanId(line.spanId)) {
    return false
  }

  return (
    logLineMatchesLevelFilter(line, filterSet) &&
    logLineMatchesTermFilter(line, filterSet)
  )
}

export function filterLogLinesForDisplay(
  lines: LogLine[],
  filterSet: FilterSet
): LogLine[] {
  let result: LogLine[] = []
  let prologuesBySpanId: { [key: string]: LogLine[] } = {}
  let shouldDisplayPrologues = filterSet.level !== FilterLevel.all

  function trackPrologueLine(line: LogLine) {
    if (!prologuesBySpanId[line.spanId]) {
      prologuesBySpanId[line.spanId] = []
    }
    prologuesBySpanId[line.spanId].push(line)
  }

  function getAndClearPrologue(spanId: string): LogLine[] {
    let spanLines = prologuesBySpanId[spanId]
    if (!spanLines) {
      return []
    }

    delete prologuesBySpanId[spanId]
    return spanLines.slice(-DISPLAY_LOG_PROLOGUE_LENGTH)
  }

  lines.forEach((line) => {
    let matches = logLineMatchesDisplayFilter(line, filterSet)
    if (matches) {
      if (shouldDisplayPrologues) {
        result.push(...getAndClearPrologue(line.spanId))
      }
      result.push(line)
    } else if (shouldDisplayPrologues) {
      trackPrologueLine(line)
    }
  })

  return result
}
