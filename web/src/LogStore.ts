// Client-side log store, which helps client side rendering and filtering of logs.
//
// Loosely adapted from the data structures in
// pkg/model/logstore/logstore.go
// but with better support for incremental updates and rendering.

import { LogLine } from "./types"
import { isBuildSpanId } from "./logs"

// Firestore doesn't properly handle maps with keys equal to the empty string, so
// we normalize all empty span ids to '_' client-side.
const defaultSpanId = "_"
const fieldNameProgressId = "progressID"

type LogSpan = {
  spanId: string
  manifestName: string
  firstLineIndex: number
  lastLineIndex: number
}

type LogWarning = {
  anchorIndex: number
  spanId: string
  text: string
}

class StoredLine {
  spanId: string
  time: string
  text: string
  level: string
  anchor: boolean
  fields: { [key: string]: string } | null

  constructor(seg: Proto.webviewLogSegment) {
    this.spanId = seg.spanId || defaultSpanId
    this.time = seg.time ?? ""
    this.text = seg.text ?? ""
    this.level = seg.level ?? "INFO"
    this.anchor = seg.anchor ?? false
    this.fields = (seg.fields as { [key: string]: string }) ?? null
  }

  field(key: string) {
    if (!this.fields) {
      return ""
    }
    return this.fields[key] ?? ""
  }

  isComplete() {
    return this.text[this.text.length - 1] == "\n"
  }

  canContinueLine(other: StoredLine) {
    return this.level == other.level && this.spanId == other.spanId
  }
}

class LogStore {
  // Track which segments we've received from the server.
  checkpoint: number

  spans: { [key: string]: LogSpan }

  // These are held in-memory so we can send them on snapshot, but
  // aren't used in rendering.
  segments: Proto.webviewLogSegment[]

  // As segments are appended, we fold them into our internal line-by-line model
  // for rendering.
  lines: StoredLine[]

  // A cache of the react data model
  lineCache: { [key: number]: LogLine }

  // We index all the warnings up-front by span id.
  warningIndex: { [key: string]: LogWarning[] }

  constructor() {
    this.spans = {}
    this.segments = []
    this.lines = []
    this.checkpoint = 0
    this.warningIndex = {}
    this.lineCache = {}
  }

  warnings(spanId: string): LogWarning[] {
    return this.warningIndex[spanId] ?? []
  }

  toLogList(): Proto.webviewLogList {
    let spans = {} as { [key: string]: Proto.webviewLogSpan }
    for (let key in this.spans) {
      spans[key] = { manifestName: this.spans[key].manifestName }
    }

    let segments = this.segments.map(
      (segment): Proto.webviewLogSegment => {
        let spanId = segment.spanId
        let time = segment.time
        let text = segment.text
        let level = segment.level
        let fields = segment.fields
        return { spanId, time, text, level, fields }
      }
    )
    return {
      spans: spans,
      segments: segments,
    }
  }

  append(logList: Proto.webviewLogList) {
    let newSpans = logList.spans as { [key: string]: Proto.webviewLogSpan }
    let newSegments = logList.segments ?? []
    let fromCheckpoint = logList.fromCheckpoint ?? 0
    let toCheckpoint = logList.toCheckpoint ?? 0
    if (fromCheckpoint < 0) {
      return
    }

    if (fromCheckpoint < this.checkpoint) {
      // The server is re-sending some logs we already have, so slice them off.
      let deleteCount = this.checkpoint - fromCheckpoint
      newSegments = newSegments.slice(deleteCount)
    }

    if (toCheckpoint > this.checkpoint) {
      this.checkpoint = toCheckpoint
    }

    for (let key in newSpans) {
      let spanId = key || defaultSpanId
      let existingSpan = this.spans[spanId]
      if (!existingSpan) {
        this.spans[spanId] = {
          spanId: spanId,
          manifestName: newSpans[key].manifestName ?? "",
          firstLineIndex: -1,
          lastLineIndex: -1,
        }
      }
    }

    newSegments.forEach(newSegment => {
      // workaround firestore bug. see comments on defaultSpanId.
      newSegment.spanId = newSegment.spanId || defaultSpanId
      this.segments.push(newSegment)

      let candidate = new StoredLine(newSegment)
      let spanId = candidate.spanId
      let span = this.spans[spanId]
      if (!span) {
        // If we don't have the span for this log, we can't meaningfully print it,
        // so just drop it. This means that there's a bug on the server, and
        // the best the client can do is fail gracefully.
        return
      }
      let isStartingNewLine = false
      if (span.lastLineIndex == -1) {
        isStartingNewLine = true
      } else {
        let line = this.lines[span.lastLineIndex]
        if (this.maybeOverwriteLine(candidate, span)) {
          return
        } else if (line.isComplete() || !line.canContinueLine(candidate)) {
          isStartingNewLine = true
        } else {
          line.text += candidate.text
          delete this.lineCache[span.lastLineIndex]
          return
        }
      }

      if (span.firstLineIndex == -1) {
        span.firstLineIndex = this.lines.length
      }

      if (isStartingNewLine) {
        span.lastLineIndex = this.lines.length
        this.lines.push(candidate)
      }
    })
  }

  // If this line has a progress id, see if we can overwrite a previous line.
  // Return true if we were able to overwrite the line successfully.
  private maybeOverwriteLine(candidate: StoredLine, span: LogSpan): boolean {
    let progressId = candidate.field(fieldNameProgressId)
    if (!progressId) {
      return false
    }

    // Iterate backwards and figure out which line to overwrite.
    for (let i = span.lastLineIndex; i >= span.firstLineIndex; i--) {
      let cur = this.lines[i]
      if (cur.spanId != candidate.spanId) {
        // skip lines from other spans
        // TODO(nick): maybe we should track if spans are interleaved, and rearrange the
        // lines to make more sense?
        continue
      }

      // If we're outside the "progress" zone, we couldn't find it.
      let curProgressId = cur.field(fieldNameProgressId)
      if (!curProgressId) {
        return false
      }

      if (progressId != curProgressId) {
        continue
      }

      cur.text = candidate.text
      delete this.lineCache[i]
      return true
    }
    return false
  }

  allLog(): LogLine[] {
    return this.logHelper(this.spans)
  }

  spanLog(spanIds: string[]): LogLine[] {
    let spans: { [key: string]: LogSpan } = {}
    spanIds.forEach(spanId => {
      spanId = spanId ? spanId : defaultSpanId
      let span = this.spans[spanId]
      if (span) {
        spans[spanId] = span
      }
    })

    return this.logHelper(spans)
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

  getOrderedBuildSpanIds(spanId: string): string[] {
    let startSpan = this.spans[spanId]
    if (!startSpan) {
      return []
    }

    let manifestName = startSpan.manifestName
    let spansById: { [key: string]: LogSpan } = {}
    for (let key in this.spans) {
      if (!isBuildSpanId(key)) {
        continue
      }

      let span = this.spans[key]
      if (span.manifestName != manifestName) {
        continue
      }

      spansById[key] = span
    }

    return Object.keys(spansById).sort((a, b) => {
      return spansById[a].firstLineIndex - spansById[b].firstLineIndex
    })
  }

  getOrderedBuildSpans(spanId: string): LogSpan[] {
    return this.getOrderedBuildSpanIds(spanId).map(spanId => this.spans[spanId])
  }

  // Given a build span in the current manifest, find the next build span.
  nextBuildSpan(spanId: string): LogSpan | null {
    let spanIds = this.getOrderedBuildSpanIds(spanId)
    let currentIndex = spanIds.indexOf(spanId)
    if (currentIndex == -1 || currentIndex == spanIds.length - 1) {
      return null
    }
    return this.spans[spanIds[currentIndex + 1]]
  }

  // Find all the logs "caused" by a particular build.
  //
  // Eventually, we should add causality links between spans to the
  // data model itself! c.f., Links in open-tracing
  // https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/api-tracing.md#add-links
  // But for now, we just hack some spans together based on their manifest name
  // and when they showed up.
  traceLog(spanId: string): LogLine[] {
    // Currently, we only support tracing of build logs.
    if (!isBuildSpanId(spanId)) {
      return []
    }

    let startSpan = this.spans[spanId]
    let spans: { [key: string]: LogSpan } = {}
    spans[spanId] = startSpan

    let nextBuildSpan = this.nextBuildSpan(spanId)

    // Grab all the spans that start between this span and the next build.
    //
    // TODO(nick): This currently skips any events that happen
    // because they're part of an "events" span where the causality
    // is uncertain. We should be more intelligent about sucking in events.
    for (let key in this.spans) {
      let candidate = this.spans[key]
      if (candidate.manifestName != startSpan.manifestName) {
        continue
      }

      if (
        candidate.firstLineIndex > startSpan.firstLineIndex &&
        (nextBuildSpan == null ||
          candidate.firstLineIndex < nextBuildSpan.firstLineIndex)
      ) {
        spans[key] = candidate
      }
    }

    return this.logHelper(spans)
  }

  manifestLog(mn: string): LogLine[] {
    let spans = this.spansForManifest(mn)
    return this.logHelper(spans)
  }

  logHelper(spansToLog: { [key: string]: LogSpan }): LogLine[] {
    let result: LogLine[] = []

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
    let lastIndex = this.lines.length - 1
    let isFilteredLog =
      Object.keys(spansToLog).length != Object.keys(this.spans).length
    if (isFilteredLog) {
      let earliestStartIndex = -1
      let latestEndIndex = -1
      for (let spanId in spansToLog) {
        let span = spansToLog[spanId]
        if (
          earliestStartIndex == -1 ||
          (span.firstLineIndex !== -1 &&
            span.firstLineIndex < earliestStartIndex)
        ) {
          earliestStartIndex = span.firstLineIndex
        }
        if (
          latestEndIndex == -1 ||
          (span.lastLineIndex !== -1 && span.lastLineIndex > latestEndIndex)
        ) {
          latestEndIndex = span.lastLineIndex
        }
      }

      if (earliestStartIndex == -1) {
        return []
      }

      startIndex = earliestStartIndex
      lastIndex = latestEndIndex
    }

    let currentLine = {}
    for (let i = startIndex; i <= lastIndex; i++) {
      let storedLine = this.lines[i]
      let spanId = storedLine.spanId
      let span = spansToLog[spanId]
      if (!span) {
        continue
      }

      let line = this.lineCache[i]
      if (!line) {
        let text = storedLine.text
        // strip off the newline
        if (text[text.length - 1] == "\n") {
          text = text.substring(0, text.length - 1)
        }
        line = {
          text: text,
          level: storedLine.level,
          manifestName: span.manifestName,
          buildEvent: storedLine.fields?.buildEvent,
          spanId: spanId,
        }

        this.lineCache[i] = line
      }

      result.push(line)
    }

    return result
  }
}

export default LogStore
