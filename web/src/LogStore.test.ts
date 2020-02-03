import LogStore from "./LogStore"
import { logLinesToString } from "./logs"

describe("LogStore", () => {
  function now() {
    return new Date().toString()
  }

  function newGlobalSegment(text: string): Proto.webviewLogSegment {
    return { text: text, time: now() }
  }
  function newGlobalLevelSegment(
    level: string,
    text: string
  ): Proto.webviewLogSegment {
    return { level: level, text: text, time: now() }
  }
  function newManifestSegment(
    name: string,
    text: string
  ): Proto.webviewLogSegment {
    return { spanId: name, text: text, time: now() }
  }

  it("handles simple printing", () => {
    let logs = new LogStore()
    logs.append({
      spans: { "": {} },
      segments: [newGlobalSegment("foo"), newGlobalSegment("bar")],
    })

    expect(logLinesToString(logs.allLog(), true)).toEqual("foobar")
  })

  it("handles caching", () => {
    let logs = new LogStore()
    logs.append({
      spans: { "": {} },
      segments: [newGlobalSegment("foo")],
      fromCheckpoint: 0,
      toCheckpoint: 1,
    })

    expect(logLinesToString(logs.allLog(), true)).toEqual("foo")
    expect(logs.lineCache[0].text).toEqual("foo")
    expect(logs.allLog()[0]).toStrictEqual(logs.allLog()[0])

    logs.append({
      spans: { "": {} },
      segments: [newGlobalSegment("bar")],
      fromCheckpoint: 1,
      toCheckpoint: 2,
    })

    expect(logLinesToString(logs.allLog(), true)).toEqual("foobar")
    expect(logs.lineCache[0].text).toEqual("foobar")
  })

  it("handles changing levels", () => {
    let logs = new LogStore()
    logs.append({
      spans: { "": {} },
      segments: [
        newGlobalLevelSegment("INFO", "foo"),
        newGlobalLevelSegment("DEBUG", "bar"),
        newGlobalLevelSegment("INFO", "baz"),
      ],
    })

    expect(logLinesToString(logs.allLog(), true)).toEqual("foo\nbar\nbaz")
  })

  it("handles prefixes in all logs", () => {
    let logs = new LogStore()
    logs.append({
      spans: { "": {}, fe: { manifestName: "fe" } },
      segments: [
        newGlobalSegment("line1\n"),
        newManifestSegment("fe", "line2\n"),
        newGlobalSegment("line3\n"),
      ],
    })

    expect(logLinesToString(logs.allLog(), true)).toEqual(
      "line1\nfe          ┊ line2\nline3"
    )
  })

  it("handles long-prefixes", () => {
    let logs = new LogStore()
    logs.append({
      spans: {
        "": {},
        "cockroachdb-frontend": { manifestName: "cockroachdb-frontend" },
      },
      segments: [
        newGlobalSegment("line1\n"),
        newManifestSegment("cockroachdb-frontend", "line2\n"),
        newGlobalSegment("line3\n"),
      ],
    })

    expect(logLinesToString(logs.allLog(), true)).toEqual(
      "line1\ncockroachdb…┊ line2\nline3"
    )
  })

  it("handles manifest logs", () => {
    let logs = new LogStore()
    logs.append({
      spans: { "": {}, fe: { manifestName: "fe" } },
      segments: [
        newGlobalSegment("line1\n"),
        newManifestSegment("fe", "line2\n"),
        newGlobalSegment("line3\n"),
      ],
    })

    expect(logLinesToString(logs.manifestLog("fe"), false)).toEqual("line2")
  })

  it("handles multi-span manifest logs", () => {
    let logs = new LogStore()
    logs.append({
      spans: {
        "pod-a": { manifestName: "fe" },
        "pod-b": { manifestName: "fe" },
      },
      segments: [
        { spanId: "pod-a", text: "pod-a: line1\n", time: now() },
        { spanId: "pod-b", text: "pod-b: line2\n", time: now() },
        { spanId: "pod-a", text: "pod-a: line3\n", time: now() },
      ],
    })

    expect(logLinesToString(logs.manifestLog("fe"), false)).toEqual(
      "pod-a: line1\npod-b: line2\npod-a: line3"
    )
    expect(logLinesToString(logs.spanLog(["pod-a", "pod-b"]), false)).toEqual(
      "pod-a: line1\npod-b: line2\npod-a: line3"
    )
    expect(logLinesToString(logs.spanLog(["pod-a"]), false)).toEqual(
      "pod-a: line1\npod-a: line3"
    )
    expect(logLinesToString(logs.spanLog(["pod-b"]), false)).toEqual(
      "pod-b: line2"
    )
    expect(logLinesToString(logs.spanLog(["pod-b"]), false)).toEqual(
      "pod-b: line2"
    )
    expect(logLinesToString(logs.spanLog(["pod-c"]), false)).toEqual("")
  })

  it("handles incremental logs", () => {
    let logs = new LogStore()
    logs.append({
      spans: { "": {} },
      segments: [newGlobalSegment("line1\n"), newGlobalSegment("line2\n")],
      fromCheckpoint: 0,
      toCheckpoint: 2,
    })
    logs.append({
      spans: { "": {} },
      segments: [newGlobalSegment("line3\n"), newGlobalSegment("line4\n")],
      fromCheckpoint: 2,
      toCheckpoint: 4,
    })
    logs.append({
      spans: { "": {} },
      segments: [
        newGlobalSegment("line4\n"),
        newGlobalSegment("line4\n"),
        newGlobalSegment("line5\n"),
      ],
      fromCheckpoint: 2,
      toCheckpoint: 5,
    })
    expect(logLinesToString(logs.allLog(), true)).toEqual(
      "line1\nline2\nline3\nline4\nline5"
    )
  })

  it("handles progressLogs interleaved", () => {
    let logs = new LogStore()
    logs.append({
      spans: { "": {} },
      segments: [
        { text: "layer 1: Pending\n", fields: { progressID: "layer 1" } },
        { text: "layer 2: Pending\n", fields: { progressID: "layer 2" } },
        { text: "layer 3: Pending\n", fields: { progressID: "layer 3" } },
      ],
      fromCheckpoint: 0,
      toCheckpoint: 3,
    })

    expect(logLinesToString(logs.allLog(), true)).toEqual(
      "layer 1: Pending\nlayer 2: Pending\nlayer 3: Pending"
    )

    logs.append({
      segments: [
        { text: "layer 2: Finished\n", fields: { progressID: "layer 2" } },
      ],
      fromCheckpoint: 3,
      toCheckpoint: 4,
    })

    expect(logLinesToString(logs.allLog(), true)).toEqual(
      "layer 1: Pending\nlayer 2: Finished\nlayer 3: Pending"
    )
  })

  it("handles progressLogs adjacent", () => {
    let logs = new LogStore()
    logs.append({
      spans: { "": {} },
      segments: [
        { text: "layer 1: Pending\n", fields: { progressID: "layer 1" } },
      ],
      fromCheckpoint: 0,
      toCheckpoint: 1,
    })

    expect(logLinesToString(logs.allLog(), true)).toEqual("layer 1: Pending")

    logs.append({
      segments: [
        { text: "layer 1: Finished\n", fields: { progressID: "layer 1" } },
      ],
      fromCheckpoint: 1,
      toCheckpoint: 2,
    })

    expect(logLinesToString(logs.allLog(), true)).toEqual("layer 1: Finished")
  })
})
