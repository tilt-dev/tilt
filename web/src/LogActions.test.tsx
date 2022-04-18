import { render, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import React from "react"
import { AnalyticsAction } from "./analytics"
import {
  cleanupMockAnalyticsCalls,
  expectIncrs,
  mockAnalyticsCalls,
} from "./analytics_test_helpers"
import {
  LogFontSizeScaleCSSProperty,
  LogFontSizeScaleLocalStorageKey,
  LogFontSizeScaleMinimumPercentage,
  LogsFontSize,
} from "./LogActions"

describe("LogsFontSize", () => {
  beforeEach(() => {
    // CSS won't be loaded in test context, so just explicitly set it
    document.documentElement.style.setProperty("--log-font-scale", "100%")
    mockAnalyticsCalls()
  })
  afterEach(() => {
    localStorage.clear()
    document.documentElement.style.removeProperty("--log-font-scale")
    cleanupMockAnalyticsCalls()
  })

  const getCSSValue = () =>
    getComputedStyle(document.documentElement).getPropertyValue(
      LogFontSizeScaleCSSProperty
    )
  // react-storage-hooks JSON (de)serializes transparently,
  // need to do the same when directly manipulating local storage
  const getLocalStorageValue = () =>
    JSON.parse(localStorage.getItem(LogFontSizeScaleLocalStorageKey) || "")
  const setLocalStorageValue = (val: string) =>
    localStorage.setItem(LogFontSizeScaleLocalStorageKey, JSON.stringify(val))

  it("restores persisted font scale on load", () => {
    setLocalStorageValue("360%")
    render(<LogsFontSize />)
    expect(getCSSValue()).toEqual("360%")
  })

  it("decreases font scale", async () => {
    render(<LogsFontSize />)
    userEvent.click(screen.getByLabelText("Decrease log font size"))

    await waitFor(() => {
      expect(getCSSValue()).toEqual("95%")
    })
    expect(getLocalStorageValue()).toEqual(`95%`) // JSON serialized
    expectIncrs({
      name: "ui.web.zoomLogs",
      tags: { action: AnalyticsAction.Click, dir: "out" },
    })
  })

  it("has a minimum font scale", async () => {
    setLocalStorageValue(`${LogFontSizeScaleMinimumPercentage}%`)
    render(<LogsFontSize />)
    userEvent.click(screen.getByLabelText("Decrease log font size"))

    await waitFor(() => {
      expect(getCSSValue()).toEqual("10%")
    })
    expect(getLocalStorageValue()).toEqual(`10%`)
  })

  it("increases font scale", async () => {
    render(<LogsFontSize />)
    userEvent.click(screen.getByLabelText("Increase log font size"))

    await waitFor(() => {
      expect(getCSSValue()).toEqual("105%")
    })
    expect(getLocalStorageValue()).toEqual(`105%`)
    expectIncrs({
      name: "ui.web.zoomLogs",
      tags: { action: AnalyticsAction.Click, dir: "in" },
    })
  })
})
