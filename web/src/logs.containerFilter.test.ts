import { EMPTY_FILTER_TERM, FilterLevel, FilterSource } from "./logfilters"
import { filterLogLinesForDisplay } from "./logs"
import { LogLine } from "./types"

function makeLine(overrides: Partial<LogLine> = {}): LogLine {
  return {
    text: "some log text\n",
    manifestName: "my-resource",
    level: "INFO",
    spanId: "span1",
    storedLineIndex: 0,
    ...overrides,
  }
}

const baseFilterSet = {
  level: FilterLevel.all,
  source: FilterSource.all,
  term: EMPTY_FILTER_TERM,
  containers: [] as string[],
}

describe("LogDisplay container filter", () => {
  it("passes all lines through when containers is empty", () => {
    const lines = [
      makeLine({ containerName: "app" }),
      makeLine({ containerName: "istio-proxy" }),
      makeLine({ containerName: undefined }),
    ]
    const result = filterLogLinesForDisplay(lines, {
      ...baseFilterSet,
      containers: [],
    })
    expect(result).toHaveLength(3)
  })

  it("filters to only the selected containers", () => {
    const lines = [
      makeLine({ text: "app log\n", containerName: "app" }),
      makeLine({ text: "istio log\n", containerName: "istio-proxy" }),
      makeLine({ text: "dapr log\n", containerName: "daprd" }),
    ]
    const result = filterLogLinesForDisplay(lines, {
      ...baseFilterSet,
      containers: ["app"],
    })
    expect(result).toHaveLength(1)
    expect(result[0].text).toBe("app log\n")
  })

  it("passes through lines with no containerName even when filter is active", () => {
    const lines = [
      makeLine({ text: "build log\n", containerName: undefined }),
      makeLine({ text: "app log\n", containerName: "app" }),
      makeLine({ text: "istio log\n", containerName: "istio-proxy" }),
    ]
    const result = filterLogLinesForDisplay(lines, {
      ...baseFilterSet,
      containers: ["app"],
    })
    // build log (no container) and app log should both pass through
    expect(result).toHaveLength(2)
    expect(result.map((l) => l.text)).toEqual(["build log\n", "app log\n"])
  })

  it("passes through buildEvent lines even when filter is active", () => {
    const lines = [
      makeLine({
        text: "build event\n",
        buildEvent: "init",
        containerName: undefined,
      }),
      makeLine({ text: "app log\n", containerName: "app" }),
      makeLine({ text: "istio log\n", containerName: "istio-proxy" }),
    ]
    const result = filterLogLinesForDisplay(lines, {
      ...baseFilterSet,
      containers: ["app"],
    })
    expect(result).toHaveLength(2)
    expect(result.map((l) => l.text)).toEqual(["build event\n", "app log\n"])
  })

  it("supports multiple selected containers", () => {
    const lines = [
      makeLine({ text: "app log\n", containerName: "app" }),
      makeLine({ text: "istio log\n", containerName: "istio-proxy" }),
      makeLine({ text: "dapr log\n", containerName: "daprd" }),
    ]
    const result = filterLogLinesForDisplay(lines, {
      ...baseFilterSet,
      containers: ["app", "daprd"],
    })
    expect(result).toHaveLength(2)
    expect(result.map((l) => l.text)).toEqual(["app log\n", "dapr log\n"])
  })
})
