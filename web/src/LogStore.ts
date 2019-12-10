// Client-side log store, which helps client side rendering and filtering of logs.
//
// Loosely adapted from the data structures in
// pkg/model/logstore/logstore.go
// but with better support for incremental updates and rendering.

import { LogLine } from "./types"

type LogSpan = {
  manifestName: string
  lastSegmentIndex: number
  firstSegmentIndex: number
}

class LogSegment {
  spanId: string
  time: string
  text: string
  level: string
  continuesLine: boolean

  constructor(seg: Proto.webviewLogSegment) {
    this.spanId = seg.spanId ?? ""
    this.time = seg.time ?? ""
    this.text = seg.text ?? ""
    this.level = seg.level ?? "INFO"
    this.continuesLine = false
  }

  startsLine() {
    return !this.continuesLine
  }

  isComplete() {
    return this.text[this.text.length - 1] == "\n"
  }

  canContinueLine(seg: LogSegment) {
    return this.level == seg.level && this.spanId == seg.spanId
  }
}

class LogStore {
  spans: { [key: string]: LogSpan }
  segments: LogSegment[]

  constructor() {
    this.spans = {}
    this.segments = []
  }

  append(logList: Proto.webviewLogList) {
    let newSpans = logList.spans as { [key: string]: Proto.webviewLogSpan }
    for (let key in newSpans) {
      let exists = this.spans[key]
      if (!exists) {
        this.spans[key] = {
          manifestName: newSpans[key].manifestName ?? "",
          firstSegmentIndex: -1,
          lastSegmentIndex: -1,
        }
      }
    }

    let segments = logList.segments ?? []
    segments.forEach(segment => {
      let newSegment = new LogSegment(segment)
      let span = this.spans[newSegment.spanId]
      if (!span) {
        // If we don't have the span for this log, we can't meaningfully print it,
        // so just drop it. This means that there's a bug on the server, and
        // the best the client can do is fail gracefully.
        return
      }
      let isStartingNewLine = false
      if (span.lastSegmentIndex == -1) {
        isStartingNewLine = true
      } else {
        let seg = this.segments[span.lastSegmentIndex]
        if (seg.isComplete() || !seg.canContinueLine(newSegment)) {
          isStartingNewLine = true
        }
      }

      if (span.firstSegmentIndex == -1) {
        span.firstSegmentIndex = this.segments.length
      }
      span.lastSegmentIndex = this.segments.length

      newSegment.continuesLine = !isStartingNewLine
      this.segments.push(newSegment)
    })
  }

  allLog(): LogLine[] {
    return this.manifestLog("")
  }

  spansForManifest(mn: string): { [key: string]: LogSpan } {
    let result: { [key: string]: LogSpan } = {}
    for (let spanId in this.spans) {
      let span = this.spans[spanId]
      if (span.manifestName == mn) {
        result[spanId] = span
      }
    }
    return result
  }

  manifestLog(mn: string): LogLine[] {
    let result: LogLine[] = []
    let lastLineCompleted = false
    let allLogs = mn === ""
    let filteredLogs = mn !== ""

    // We want to print the log line-by-line, but we don't actually store the logs
    // line-by-line. We store them as segments.
    //
    // This means we need to:
    // 1) At segment x,
    // 2) If x starts a new line, print it, then run ahead to print the rest of the line
    //    until the entire line is consumed.
    // 3) If x does not start a new line, skip it, because we assume it was handled
    //    in a previous line.
    //
    // This can have some O(n^2) perf characteristics in the worst case, but
    // for normal inputs should be fine.
    let startIndex = 0
    let lastIndex = this.segments.length - 1
    if (filteredLogs) {
      let spans = this.spansForManifest(mn)
      let earliestStartIndex = -1
      let latestEndIndex = -1
      for (let spanId in spans) {
        let span = spans[spanId]
        if (
          earliestStartIndex == -1 ||
          span.firstSegmentIndex < earliestStartIndex
        ) {
          earliestStartIndex = span.firstSegmentIndex
        }
        if (latestEndIndex == -1 || span.lastSegmentIndex > latestEndIndex) {
          latestEndIndex = span.lastSegmentIndex
        }
      }

      if (earliestStartIndex == -1) {
        return []
      }

      startIndex = earliestStartIndex
      lastIndex = latestEndIndex
    }

    let isFirstLine = true
    let currentLine = {}
    for (let i = startIndex; i <= lastIndex; i++) {
      let segment = this.segments[i]
      if (!segment.startsLine()) {
        continue
      }

      let spanId = segment.spanId
      let span = this.spans[spanId]
      if (!span) {
        // Something has gone terribly wrong
        continue
      }

      if (filteredLogs && mn != span.manifestName) {
        continue
      }

      let currentLine = { manifestName: span.manifestName, text: segment.text }
      isFirstLine = false

      // If this segment is not complete, run ahead and try to complete it.
      if (segment.isComplete()) {
        lastLineCompleted = true

        // strip off the newline
        currentLine.text = currentLine.text.substring(
          0,
          currentLine.text.length - 1
        )
        result.push(currentLine)
        continue
      }

      lastLineCompleted = false
      for (
        let currentIndex = i + 1;
        currentIndex <= span.lastSegmentIndex;
        currentIndex++
      ) {
        let currentSeg = this.segments[currentIndex]
        if (currentSeg.spanId != spanId) {
          continue
        }

        if (!currentSeg.canContinueLine(segment)) {
          break
        }

        currentLine.text += currentSeg.text
        if (currentSeg.isComplete()) {
          lastLineCompleted = true

          // strip off the newline
          currentLine.text = currentLine.text.substring(
            0,
            currentLine.text.length - 1
          )
          break
        }
      }
      result.push(currentLine)
    }

    return result
  }
}

export default LogStore
