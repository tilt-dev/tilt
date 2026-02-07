import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import React from "react"
import CopyLogs from "./CopyLogs"
import { logLinesToString } from "./logs"
import LogStore, { LogStoreProvider } from "./LogStore"
import { appendLinesForManifestAndSpan } from "./testlogs"
import { ResourceName } from "./types"

describe("CopyLogs", () => {
  let writeTextMock: jest.Mock

  beforeEach(() => {
    writeTextMock = jest.fn().mockResolvedValue(undefined)
    Object.assign(navigator, {
      clipboard: {
        writeText: writeTextMock,
      },
    })
  })

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

  it("copies all logs to clipboard", () => {
    const logStore = createPopulatedLogStore()
    render(
      <LogStoreProvider value={logStore}>
        <CopyLogs resourceName={ResourceName.all} />
      </LogStoreProvider>
    )

    userEvent.click(screen.getByRole("button"))

    const expectedText = logLinesToString(logStore.allLog(), false)
    expect(writeTextMock).toHaveBeenCalledWith(expectedText)
  })

  it("copies logs for a specific resource to clipboard", () => {
    const logStore = createPopulatedLogStore()
    render(
      <LogStoreProvider value={logStore}>
        <CopyLogs resourceName={"vigoda"} />
      </LogStoreProvider>
    )

    userEvent.click(screen.getByRole("button"))

    const expectedText = logLinesToString(logStore.manifestLog("vigoda"), true)
    expect(writeTextMock).toHaveBeenCalledWith(expectedText)
  })

  it("does not modify the log store", () => {
    const logStore = createPopulatedLogStore()
    const logsBefore = logLinesToString(logStore.allLog(), false)

    render(
      <LogStoreProvider value={logStore}>
        <CopyLogs resourceName={ResourceName.all} />
      </LogStoreProvider>
    )

    userEvent.click(screen.getByRole("button"))

    const logsAfter = logLinesToString(logStore.allLog(), false)
    expect(logsAfter).toEqual(logsBefore)
  })
})
