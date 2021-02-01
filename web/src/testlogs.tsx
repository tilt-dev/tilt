import LogStore from "./LogStore"

export type Line = string | { text: string; fields?: any }

function now() {
  return new Date().toString()
}

// Adds lines to a log store.
// We accept lines expressed as var args or as an array.
// (Expressing them as var args can hit call stack maximums if you're not careful).
export function appendLines(
  logStore: LogStore,
  name: string,
  ...lineOrList: Line[] | Line[][]
) {
  appendLinesForManifestAndSpan(logStore, name, name, ...lineOrList)
}

// Adds lines to a log store.
// We accept lines expressed as var args or as an array.
// (Expressing them as var args can hit call stack maximums if you're not careful).
export function appendLinesForManifestAndSpan(
  logStore: LogStore,
  manifestName: string,
  spanId: string,
  ...lineOrList: Line[] | Line[][]
) {
  let lines = lineOrList.flat()
  let fromCheckpoint = logStore.checkpoint
  let toCheckpoint = fromCheckpoint + lines.length

  let spans = {} as any
  spanId = spanId || "_"
  spans[spanId] = { manifestName: manifestName }

  let segments = []
  for (let line of lines) {
    let obj = { time: now(), spanId: spanId, text: "" } as any
    if (typeof line == "string") {
      obj.text = line
    } else {
      for (let key in line) {
        obj[key] = (line as any)[key]
      }
    }
    segments.push(obj)
  }

  logStore.append({ spans, segments, fromCheckpoint, toCheckpoint })
}
