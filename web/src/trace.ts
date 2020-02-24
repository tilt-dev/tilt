import { LogTraceNav, LogTrace } from "./types"
import { isBuildSpanId } from "./logs"
import PathBuilder from "./PathBuilder"
import LogStore from "./LogStore"

function traceData(
  pb: PathBuilder,
  span: { spanId: string; manifestName: string },
  index: number
): LogTrace {
  let url = pb.path(`/r/${span.manifestName}/trace/${span.spanId}`)
  return { url, index }
}

// Build navigational data for the trace we're currently looking at.
function traceNav(
  logStore: LogStore,
  pb: PathBuilder,
  spanId: string
): LogTraceNav | null {
  // Currently, we only support tracing of build logs.
  if (!isBuildSpanId(spanId) || !logStore) {
    return null
  }

  let spans = logStore.getOrderedBuildSpans(spanId)
  let currentIndex = spans.findIndex(span => span.spanId == spanId)
  let span = spans[currentIndex]
  if (!span) {
    return null
  }
  let nav: LogTraceNav = {
    count: spans.length,
    current: traceData(pb, span, currentIndex),
  }

  if (currentIndex < spans.length - 1) {
    nav.next = traceData(pb, spans[currentIndex + 1], currentIndex + 1)
  }
  if (currentIndex > 0) {
    nav.prev = traceData(pb, spans[currentIndex - 1], currentIndex - 1)
  }
  return nav
}

export { traceData, traceNav }
