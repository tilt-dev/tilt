import { act, render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import React from "react"
import CopyLogs, { copyLogs } from "./CopyLogs"
import { logLinesToString, stripAnsiCodes } from "./logs"
import LogStore, { LogStoreProvider } from "./LogStore"
import { appendLinesForManifestAndSpan } from "./testlogs"
import { ResourceName } from "./types"

describe("CopyLogs", () => {
  let writeTextMock: jest.Mock

  beforeEach(() => {
    jest.useFakeTimers()
    writeTextMock = jest.fn().mockResolvedValue(undefined)
    Object.assign(navigator, {
      clipboard: {
        writeText: writeTextMock,
      },
    })
  })

  afterEach(() => {
    jest.useRealTimers()
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

    userEvent.click(screen.getByText("Copy All Logs"))

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

    userEvent.click(screen.getByText("Copy Logs"))

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

    userEvent.click(screen.getByText("Copy All Logs"))

    const logsAfter = logLinesToString(logStore.allLog(), false)
    expect(logsAfter).toEqual(logsBefore)
  })

  it("shows a tooltip with the number of copied lines", () => {
    const logStore = createPopulatedLogStore()
    render(
      <LogStoreProvider value={logStore}>
        <CopyLogs resourceName={ResourceName.all} />
      </LogStoreProvider>
    )

    userEvent.click(screen.getByText("Copy All Logs"))

    const lineCount = logStore.allLog().length
    expect(screen.getByText(`Copied ${lineCount} lines`)).toBeInTheDocument()
  })

  it("hides the tooltip after a delay", () => {
    const logStore = createPopulatedLogStore()
    render(
      <LogStoreProvider value={logStore}>
        <CopyLogs resourceName={ResourceName.all} />
      </LogStoreProvider>
    )

    userEvent.click(screen.getByText("Copy All Logs"))
    const lineCount = logStore.allLog().length
    expect(screen.getByText(`Copied ${lineCount} lines`)).toBeVisible()

    // Advance past the 1500ms dismiss timeout plus MUI tooltip exit transition
    act(() => {
      jest.advanceTimersByTime(3000)
    })

    // After the timeout, the tooltip transitions out (opacity: 0)
    expect(
      screen.queryByText(`Copied ${lineCount} lines`)
    ).not.toBeVisible()
  })

  it("returns the number of lines copied", () => {
    const logStore = createPopulatedLogStore()
    const count = copyLogs(logStore, ResourceName.all)
    expect(count).toBe(logStore.allLog().length)
  })
})

describe("stripAnsiCodes", () => {
  it("removes basic color codes", () => {
    expect(stripAnsiCodes("\x1b[31mred text\x1b[0m")).toBe("red text")
  })

  it("removes multiple ANSI sequences", () => {
    expect(
      stripAnsiCodes("\x1b[1m\x1b[32mbold green\x1b[0m normal")
    ).toBe("bold green normal")
  })

  it("removes 256-color codes", () => {
    expect(stripAnsiCodes("\x1b[38;5;196mred\x1b[0m")).toBe("red")
  })

  it("removes OSC sequences (title setting)", () => {
    expect(stripAnsiCodes("\x1b]0;window title\x07some text")).toBe(
      "some text"
    )
  })

  it("returns plain text unchanged", () => {
    expect(stripAnsiCodes("hello world")).toBe("hello world")
  })

  it("handles empty string", () => {
    expect(stripAnsiCodes("")).toBe("")
  })
})
