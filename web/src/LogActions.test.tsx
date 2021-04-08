import { mount } from "enzyme"
import React from "react"
import {
  cleanupMockAnalyticsCalls,
  expectIncrs,
  mockAnalyticsCalls,
} from "./analytics_test_helpers"
import {
  FontSizeDecreaseButton,
  FontSizeIncreaseButton,
  LogFontSizeScaleCSSProperty,
  LogFontSizeScaleLocalStorageKey,
  LogFontSizeScaleMinimumPercentage,
  LogsFontSize,
} from "./LogActions"

describe("LogsFontSize", () => {
  const cleanup = () => {
    localStorage.clear()
    document.documentElement.style.removeProperty("--log-font-scale")
    cleanupMockAnalyticsCalls()
  }

  beforeEach(() => {
    cleanup()
    // CSS won't be loaded in test context, so just explicitly set it
    document.documentElement.style.setProperty("--log-font-scale", "100%")
    mockAnalyticsCalls()
  })
  afterEach(cleanup)

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
    mount(<LogsFontSize />)
    expect(getCSSValue()).toEqual("360%")
  })

  it("decreases font scale", () => {
    const root = mount(<LogsFontSize />)
    root.find(FontSizeDecreaseButton).simulate("click")
    expect(getCSSValue()).toEqual("95%")
    expect(getLocalStorageValue()).toEqual(`95%`) // JSON serialized
    expectIncrs({
      name: "ui.web.zoomLogs",
      tags: { action: "click", dir: "out" },
    })
  })

  it("has a minimum font scale", () => {
    setLocalStorageValue(`${LogFontSizeScaleMinimumPercentage}%`)
    const root = mount(<LogsFontSize />)
    root.find(FontSizeDecreaseButton).simulate("click")
    expect(getCSSValue()).toEqual("10%")
    expect(getLocalStorageValue()).toEqual(`10%`)
  })

  it("increases font scale", () => {
    const root = mount(<LogsFontSize />)
    root.find(FontSizeIncreaseButton).simulate("click")
    expect(getCSSValue()).toEqual("105%")
    expect(getLocalStorageValue()).toEqual(`105%`)
    expectIncrs({
      name: "ui.web.zoomLogs",
      tags: { action: "click", dir: "in" },
    })
  })
})
