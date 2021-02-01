import { mount } from "enzyme"
import React from "react"
import { Alert } from "./alerts"
import ClearLogs from "./ClearLogs"
import { FilterLevel, FilterSource } from "./logfilters"
import { logLinesToString } from "./logs"
import LogStore, { LogStoreProvider } from "./LogStore"
import { oneResource } from "./testdata"
import { appendLinesForManifestAndSpan } from "./testlogs"

describe("ClearLogs", () => {
  const createAlert = (
    resourceName: string,
    source: FilterSource,
    level: FilterLevel
  ): Alert => {
    return {
      alertType: "",
      header: "",
      msg: "",
      timestamp: "",
      resourceName,
      source,
      level,
    }
  }

  const createPopulatedLogStore = (): LogStore => {
    const logStore = new LogStore()
    appendLinesForManifestAndSpan(logStore, "", "", [
      "global 1\n",
      "global 2\n",
    ])
    appendLinesForManifestAndSpan(logStore, "vigoda", "build:m1:1", [
      "m1:1 build line 1\n",
    ])
    appendLinesForManifestAndSpan(logStore, "vigoda", "pod:m1-abc123", [
      "m1:1 runtime line 1\n",
    ])
    appendLinesForManifestAndSpan(logStore, "manifest2", "build:m2", [
      "m2 build line 1\n",
    ])
    appendLinesForManifestAndSpan(logStore, "vigoda", "build:m1:2", [
      "m1:2 build line 1\n",
      "m1:2 build line 2\n",
    ])
    appendLinesForManifestAndSpan(logStore, "manifest2", "pod:m2-def456", [
      "m2 runtime line 1\n",
    ])
    return logStore
  }

  it("clears all resources", () => {
    const logStore = createPopulatedLogStore()
    const root = mount(
      <LogStoreProvider value={logStore}>
        <ClearLogs />
      </LogStoreProvider>
    )
    root.find(ClearLogs).simulate("click")
    expect(logStore.spans).toEqual({})
    expect(logStore.allLog()).toHaveLength(0)
  })

  it("clears all resources except ones with current alerts", () => {
    const logStore = createPopulatedLogStore()
    const alerts: Alert[] = [
      createAlert("vigoda", FilterSource.build, FilterLevel.warn),
      createAlert("manifest2", FilterSource.runtime, FilterLevel.error),
    ]
    const root = mount(
      <LogStoreProvider value={logStore}>
        <ClearLogs alerts={alerts} />
      </LogStoreProvider>
    )
    root.find(ClearLogs).simulate("click")
    expect(Object.keys(logStore.spans).sort()).toEqual([
      "build:m1:2",
      "pod:m2-def456",
    ])
    expect(logLinesToString(logStore.allLog(), false)).toEqual(
      "m1:2 build line 1\nm1:2 build line 2\nm2 runtime line 1"
    )
  })

  it("clears a specific resource", () => {
    const logStore = createPopulatedLogStore()
    const resource = oneResource()
    const root = mount(
      <LogStoreProvider value={logStore}>
        <ClearLogs resource={resource} />
      </LogStoreProvider>
    )
    root.find(ClearLogs).simulate("click")
    expect(Object.keys(logStore.spans).sort()).toEqual([
      "_",
      "build:m2",
      "pod:m2-def456",
    ])
    expect(logLinesToString(logStore.allLog(), false)).toEqual(
      "global 1\nglobal 2\nm2 build line 1\nm2 runtime line 1"
    )
  })
})
