import { render, RenderResult } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import React from "react"
import { AnalyticsAction } from "./analytics"
import {
  cleanupMockAnalyticsCalls,
  expectIncrs,
  mockAnalyticsCalls,
} from "./analytics_test_helpers"
import { logLinesToString } from "./logs"
import LogStore from "./LogStore"
import OverviewActionBarKeyboardShortcuts from "./OverviewActionBarKeyboardShortcuts"
import { appendLinesForManifestAndSpan } from "./testlogs"

const TEST_URL_A = { url: "https://tilt.dev:4000" }
const TEST_URL_B = { url: "https://tilt.dev:4001" }
const TEST_RESOURCE_NAME = "fake-resource"

describe("Detail View keyboard shortcuts", () => {
  let rerender: RenderResult["rerender"]
  let openEndpointMock: jest.Mock
  let logStore: LogStore

  beforeEach(() => {
    mockAnalyticsCalls()
    logStore = new LogStore()
    openEndpointMock = jest.fn()
    rerender = render(
      <OverviewActionBarKeyboardShortcuts
        logStore={logStore}
        resourceName={TEST_RESOURCE_NAME}
        openEndpointUrl={openEndpointMock}
        endpoints={[TEST_URL_A, TEST_URL_B]}
      />
    ).rerender
  })

  afterEach(() => {
    cleanupMockAnalyticsCalls()
  })

  describe("open endpoints", () => {
    it("does NOT open any endpoints if there aren't any", () => {
      rerender(
        <OverviewActionBarKeyboardShortcuts
          logStore={logStore}
          resourceName={TEST_RESOURCE_NAME}
          openEndpointUrl={openEndpointMock}
        />
      )

      userEvent.keyboard("{Shift>}1")

      expect(openEndpointMock).not.toHaveBeenCalled()
    })

    it("opens the first endpoint when SHIFT + 1 are pressed", () => {
      userEvent.keyboard("{Shift>}1")

      expect(openEndpointMock).toHaveBeenCalledWith(TEST_URL_A.url)
    })

    it("opens the corresponding endpoint when SHIFT + a number are pressed", () => {
      userEvent.keyboard("{Shift>}2")

      expect(openEndpointMock).toHaveBeenCalledWith(TEST_URL_B.url)
    })
  })

  describe("clear logs", () => {
    // Add a log to the store
    beforeEach(() =>
      appendLinesForManifestAndSpan(logStore, TEST_RESOURCE_NAME, "span:1", [
        "line 1\n",
      ])
    )

    it("clears logs when the META + BACKSPACE keys are pressed", () => {
      userEvent.keyboard("{Meta>}{Backspace}")

      expect(logLinesToString(logStore.allLog(), false)).toEqual("")
      expectIncrs({
        name: "ui.web.clearLogs",
        tags: { action: AnalyticsAction.Shortcut, all: "false" },
      })
    })

    it("clears logs when the CTRL + BACKSPACE keys are pressed", () => {
      userEvent.keyboard("{Control>}{Backspace}")

      expect(logLinesToString(logStore.allLog(), false)).toEqual("")
      expectIncrs({
        name: "ui.web.clearLogs",
        tags: { action: AnalyticsAction.Shortcut, all: "false" },
      })
    })
  })
})
