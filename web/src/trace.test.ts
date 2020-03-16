import LogStore from "./LogStore"
import PathBuilder from "./PathBuilder"
import { traceNav } from "./trace"

function now() {
  return new Date().toString()
}

function newManifestSegment(
  name: string,
  text: string
): Proto.webviewLogSegment {
  return { spanId: name, text: text, time: now() }
}

it("generates trace nav data", () => {
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
      newManifestSegment("pod:1", "pod 1 line 2\n"),
    ],
  })

  let pb = new PathBuilder("localhost:10350", "/r/fe")
  expect(traceNav(logs, pb, "build:1")).toEqual({
    count: 2,
    current: {
      index: 0,
      url: "/r/fe/trace/build:1",
    },
    next: {
      index: 1,
      url: "/r/fe/trace/build:2",
    },
  })
  expect(traceNav(logs, pb, "build:2")).toEqual({
    count: 2,
    prev: {
      index: 0,
      url: "/r/fe/trace/build:1",
    },
    current: {
      index: 1,
      url: "/r/fe/trace/build:2",
    },
  })
})
