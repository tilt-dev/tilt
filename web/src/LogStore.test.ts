import { logLinesToString } from "./logs"
import LogStore from "./LogStore"

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

  it("handles manifest spans with no segments", () => {
    let logs = new LogStore()
    logs.append({
      spans: {
        "": {},
        fe: { manifestName: "fe" },
        foo: { manifestName: "fe" },
      },
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

  it("handles last trace", () => {
    let logs = new LogStore()
    logs.append({
      spans: {
        "": {},
        "build:1": { manifestName: "fe" },
        "pod:1": { manifestName: "fe" },
        "build:2": { manifestName: "foo" },
        "pod:2": { manifestName: "foo" },
      },
      segments: [
        newManifestSegment("build:1", "build 1\n"),
        newManifestSegment("pod:1", "pod 1\n"),
        newManifestSegment("build:2", "build 2\n"),
        newManifestSegment("pod:2", "pod 2\n"),
        newManifestSegment("pod:1", "pod 1 line 2\n"),
      ],
    })

    expect(logLinesToString(logs.traceLog("build:1"), false)).toEqual(
      "build 1\npod 1\npod 1 line 2"
    )
  })

  it("handles trace ends at next build", () => {
    let logs = new LogStore()
    logs.append({
      spans: {
        "": {},
        "build:1": { manifestName: "fe" },
        "pod:1": { manifestName: "fe" },
        "build:2": { manifestName: "fe" },
        "pod:2": { manifestName: "fe" },
      },
      segments: [
        newManifestSegment("build:1", "build 1\n"),
        newManifestSegment("pod:1", "pod 1\n"),
        newManifestSegment("build:2", "build 2\n"),
        newManifestSegment("pod:2", "pod 2\n"),
      ],
    })

    expect(logLinesToString(logs.traceLog("build:1"), false)).toEqual(
      "build 1\npod 1"
    )
  })

  it("handles incremental logs", () => {
    let logs = new LogStore()
    logs.append({
      spans: {
        "": {},
        "build:1": { manifestName: "fe" },
      },
      segments: [
        newManifestSegment("build:1", "build 1\n"),
        newManifestSegment("build:1", "build 2\n"),
        newManifestSegment("build:1", "build 3\n"),
        newGlobalSegment("global line 1\n"),
      ],
      fromCheckpoint: 0,
      toCheckpoint: 4,
    })

    let patch = logs.manifestLogPatchSet("fe", 0)
    expect(logLinesToString(patch.lines, false)).toEqual(
      "build 1\nbuild 2\nbuild 3"
    )

    logs.append({
      spans: {
        "": {},
        "build:1": { manifestName: "fe" },
      },
      segments: [
        newGlobalSegment("global line 2\n"),
        newManifestSegment("build:1", "build 4\n"),
        newManifestSegment("build:1", "build 5\n"),
        newManifestSegment("build:1", "build 6\n"),
      ],
      fromCheckpoint: 4,
      toCheckpoint: 8,
    })

    let patch3 = logs.manifestLogPatchSet("fe", patch.checkpoint)
    expect(logLinesToString(patch3.lines, false)).toEqual(
      "build 4\nbuild 5\nbuild 6"
    )
  })

  it("handles incremental logs two spans", () => {
    let logs = new LogStore()
    logs.append({
      spans: {
        "build:1": { manifestName: "fe" },
      },
      segments: [
        newManifestSegment("build:1", "build 1\n"),
        newManifestSegment("build:1", "build 2\n"),
        newManifestSegment("build:1", "build 3\n"),
      ],
      fromCheckpoint: 0,
      toCheckpoint: 3,
    })

    let patch = logs.manifestLogPatchSet("fe", 0)
    expect(logLinesToString(patch.lines, false)).toEqual(
      "build 1\nbuild 2\nbuild 3"
    )

    logs.append({
      spans: {
        "build:2": { manifestName: "fe" },
      },
      segments: [
        newManifestSegment("build:2", "build 4\n"),
        newManifestSegment("build:2", "build 5\n"),
        newManifestSegment("build:2", "build 6\n"),
      ],
      fromCheckpoint: 3,
      toCheckpoint: 6,
    })

    let patch2 = logs.manifestLogPatchSet("fe", patch.checkpoint)
    expect(logLinesToString(patch2.lines, false)).toEqual(
      "build 4\nbuild 5\nbuild 6"
    )
  })

  it("handles incremental logs continuation", () => {
    let logs = new LogStore()
    logs.append({
      spans: {
        "": {},
        "build:1": { manifestName: "fe" },
      },
      segments: [
        newManifestSegment("build:1", "build 1\n"),
        newManifestSegment("build:1", "build 2\n"),
        newManifestSegment("build:1", "build ..."),
        newGlobalSegment("global line 1\n"),
      ],
      fromCheckpoint: 0,
      toCheckpoint: 4,
    })

    let patch = logs.manifestLogPatchSet("fe", 0)
    expect(logLinesToString(patch.lines, false)).toEqual(
      "build 1\nbuild 2\nbuild ..."
    )

    let patch2 = logs.manifestLogPatchSet("fe", patch.checkpoint)
    expect(patch2.lines).toEqual([])

    logs.append({
      spans: {
        "": {},
        "build:1": { manifestName: "fe" },
      },
      segments: [
        newGlobalSegment("global line 2\n"),
        newManifestSegment("build:1", "... 3\n"),
        newManifestSegment("build:1", "build 4\n"),
        newManifestSegment("build:1", "build 5\n"),
      ],
      fromCheckpoint: 4,
      toCheckpoint: 8,
    })

    let patch3 = logs.manifestLogPatchSet("fe", patch.checkpoint)
    expect(logLinesToString(patch3.lines, false)).toEqual(
      "build ...... 3\nbuild 4\nbuild 5"
    )
  })

  it("truncates output for snapshots", () => {
    let logs = new LogStore()

    logs.append({
      spans: {
        "build:1": { manifestName: "fe" },
      },
      segments: [newManifestSegment("build:1", "build 1\n")],
      fromCheckpoint: 0,
      toCheckpoint: 1,
    })

    logs.append({
      spans: {
        "build:2": { manifestName: "be" },
      },
      segments: [
        newManifestSegment("build:2", "build 2\n"),
        newManifestSegment("build:2", "build 3\n"),
      ],
      fromCheckpoint: 1,
      toCheckpoint: 3,
    })

    logs.append({
      spans: {
        "": {},
      },
      segments: [newGlobalSegment("global line 1\n")],
      fromCheckpoint: 3,
      toCheckpoint: 4,
    })

    const logList = logs.toLogList(28)
    // log should be truncated
    expect(logList.segments?.length).toEqual(2)
    // order should be preserved
    expect(logList.segments![0].text).toEqual("build 3\n")
    expect(logList.segments![1].text).toEqual("global line 1\n")

    // only spans referenced by segments in the truncated output should exist
    const spans = logList.spans as { [key: string]: Proto.webviewLogSpan }
    expect(Object.keys(spans).length).toEqual(2)
    expect(spans["build:2"].manifestName).toEqual("be")
    expect(spans["_"].manifestName).toEqual("")
  })

  it("removes manifests", () => {
    let logs = new LogStore()

    logs.append({
      spans: {
        "build:1": { manifestName: "keep" },
        "build:2": { manifestName: "purge" },
        "": {},
      },
      segments: [
        newGlobalSegment("global line 1\n"),
        newManifestSegment("build:1", "start of line - "),
        newManifestSegment("build:2", "build 2\n"),
        newManifestSegment("build:1", "middle of line - "),
        newManifestSegment("build:2", "build 3\n"),
        newManifestSegment("build:1", "end of line\n"),
      ],
      fromCheckpoint: 0,
      toCheckpoint: 4,
    })

    logs.removeSpans(["build:2"])
    expect(logs.segments).toHaveLength(4)
    expect(logs.segmentToLine).toHaveLength(4)
    expect(Object.keys(logs.spans).sort()).toEqual(["_", "build:1"])

    expect(logLinesToString(logs.allLog(), true)).toEqual(
      "global line 1\nkeep        ┊ start of line - middle of line - end of line"
    )
    expect(logLinesToString(logs.manifestLog("be"), false)).toEqual("")
  })

  it("weights on recenty and length", () => {
    let logs = new LogStore()
    expect(
      logs.heaviestManifestName({
        a: { name: "a", byteCount: 100, start: "2020-04" },
        b: { name: "b", byteCount: 100, start: "2021-04" },
      })
    ).toEqual("a")

    expect(
      logs.heaviestManifestName({
        a: { name: "a", byteCount: 100, start: "2020-04" },
        b: { name: "b", byteCount: 1000, start: "2021-04" },
      })
    ).toEqual("b")

    expect(
      logs.heaviestManifestName({
        a: { name: "a", byteCount: 100, start: "2020-04" },
        b: { name: "b", byteCount: 150, start: "2021-04" },
      })
    ).toEqual("a")
  })

  it("truncates output for snapshots", () => {
    let logs = new LogStore()
    logs.maxLogLength = 40

    logs.append({
      spans: {
        "build:1": { manifestName: "fe" },
      },
      segments: [newManifestSegment("build:1", "build 1\n")],
      fromCheckpoint: 0,
      toCheckpoint: 1,
    })

    logs.append({
      spans: {
        "build:2": { manifestName: "be" },
      },
      segments: [
        newManifestSegment("build:2", "build 2\n"),
        newManifestSegment("build:2", "build 3\n"),
        newManifestSegment("build:2", "build 4\n"),
        newManifestSegment("build:2", "build 5\n"),
        newManifestSegment("build:2", "build 6\n"),
        newManifestSegment("build:2", "build 7\n"),
      ],
      fromCheckpoint: 1,
      toCheckpoint: 7,
    })

    logs.append({
      spans: {
        "": {},
      },
      segments: [newGlobalSegment("global line 1\n")],
      fromCheckpoint: 7,
      toCheckpoint: 8,
    })

    expect(logLinesToString(logs.allLog(), true)).toEqual(
      "fe          ┊ build 1\nbe          ┊ build 7\nglobal line 1"
    )
  })
})
