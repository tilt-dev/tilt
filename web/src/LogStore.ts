// Client-side log store, which helps client side rendering and filtering of logs.
//
// Loosely adapted from the data structures in
// pkg/model/logstore/logstore.go
// but with better support for incremental updates and rendering.

type LogSpan = {
  manifestName: string
  lastSegmentIndex: number
  firstSegmentIndex: number
}

class LogSegment {
  spanId: string
  time: string
  text: string
  continuesLine: boolean

  constructor(seg: Proto.webviewLogSegment) {
    this.spanId = seg.spanId ?? ""
    this.time = seg.time ?? ""
    this.text = seg.text ?? ""
    this.continuesLine = false
  }

  startsLine() {
    return !this.continuesLine
  }

  isComplete() {
    return this.text[this.text.length - 1] == "\n"
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
      } else if (this.segments[span.lastSegmentIndex].isComplete()) {
        isStartingNewLine = true
      }

      if (span.firstSegmentIndex == -1) {
        span.firstSegmentIndex = this.segments.length
      }
      span.lastSegmentIndex = this.segments.length

      newSegment.continuesLine = !isStartingNewLine
      this.segments.push(newSegment)
    })
  }

  allLog() {
    return this.manifestLog("")
  }

  manifestLog(mn: string) {
    let lastLineCompleted = false
    let allLogs = mn === ""
    let filteredLogs = mn !== ""
    let result = ""

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
      let span = this.spans[mn]
      if (!span) {
        return ""
      }

      startIndex = span.firstSegmentIndex
      lastIndex = span.lastSegmentIndex
    }

    let isFirstLine = true
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

      // If the last segment never completed, print a newline now, so that the
      // logs from different sources don't blend together.
      if (!isFirstLine && !lastLineCompleted) {
        result += "\n"
      }

      if (allLogs && span.manifestName != "") {
        result += sourcePrefix(span.manifestName)
      }
      result += segment.text
      isFirstLine = false

      // If this segment is not complete, run ahead and try to complete it.
      if (segment.isComplete()) {
        lastLineCompleted = true
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

        result += currentSeg.text
        if (currentSeg.isComplete()) {
          lastLineCompleted = true
          break
        }
      }
    }

    return result
  }
}

function sourcePrefix(n: string) {
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

export default LogStore
