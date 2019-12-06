import LogStore from "./LogStore"

describe("LogStore", () => {
  function now() {
    return new Date().toString()
  }

  function newGlobalSegment(text: string): Proto.webviewLogSegment {
    return { text: text, time: now() }
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

    expect(logs.allLog()).toEqual("foobar")
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

    expect(logs.allLog()).toEqual("line1\nfe          ┊ line2\nline3\n")
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

    expect(logs.allLog()).toEqual("line1\ncockroachdb…┊ line2\nline3\n")
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

    expect(logs.manifestLog("fe")).toEqual("line2\n")
  })
})
