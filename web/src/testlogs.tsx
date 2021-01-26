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
  let lines = lineOrList.flat()
  let fromCheckpoint = logStore.checkpoint
  let toCheckpoint = fromCheckpoint + lines.length

  let spans = {} as any
  let spanId = name || "_"
  spans[spanId] = { manifestName: name }

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
