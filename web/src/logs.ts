// Helper functions for dealing with logs

import { FilterLevel, FilterSet, FilterSource, TermState } from "./logfilters"
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

export class LogDisplay {
  private prologuesBySpanId: { [key: string]: LogLine[] } = {}
  filterSet: FilterSet

  constructor(filterSet: FilterSet) {
    this.filterSet = filterSet
  }

  shouldDisplayPrologues(): boolean {
    return this.filterSet.level !== FilterLevel.all
  }

  matchesTermFilter(line: LogLine): boolean {
    const { term } = this.filterSet

    if (!term || term.state !== TermState.Parsed) {
      return true
    }

    return term.regexp.test(line.text)
  }

  matchesLevelFilter(line: LogLine): boolean {
    let level = this.filterSet.level
    if (level === FilterLevel.warn && line.level !== "WARN") {
      return false
    }
    if (level === FilterLevel.error && line.level !== "ERROR") {
      return false
    }
    return true
  }

  matchesFilter(line: LogLine): boolean {
    if (line.buildEvent) {
      return true
    }

    let source = this.filterSet.source
    if (source === FilterSource.runtime && isBuildSpanId(line.spanId)) {
      return false
    }
    if (source === FilterSource.build && !isBuildSpanId(line.spanId)) {
      return false
    }

    return this.matchesLevelFilter(line) && this.matchesTermFilter(line)
  }

  trackPrologueLine(line: LogLine) {
    if (!this.prologuesBySpanId[line.spanId]) {
      this.prologuesBySpanId[line.spanId] = []
    }
    this.prologuesBySpanId[line.spanId].push(line)
  }

  getAndClearPrologue(spanId: string): LogLine[] {
    let spanLines = this.prologuesBySpanId[spanId]
    if (!spanLines) {
      return []
    }

    delete this.prologuesBySpanId[spanId]
    return spanLines.slice(-DISPLAY_LOG_PROLOGUE_LENGTH)
  }

  filterLines(lines: LogLine[]): LogLine[] {
    let result: LogLine[] = []
    let shouldDisplayPrologues = this.shouldDisplayPrologues()

    lines.forEach((line) => {
      let matches = this.matchesFilter(line)
      if (matches) {
        if (shouldDisplayPrologues) {
          result.push(...this.getAndClearPrologue(line.spanId))
        }
        result.push(line)
      } else if (shouldDisplayPrologues) {
        this.trackPrologueLine(line)
      }
    })

    return result
  }
}

export function filterLogLinesForDisplay(
  lines: LogLine[],
  filterSet: FilterSet
): LogLine[] {
  return new LogDisplay(filterSet).filterLines(lines)
}
