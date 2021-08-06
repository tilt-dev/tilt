import { Action, Location } from "history"
import { AnalyticsType, navigationToTags, pathToTag } from "./analytics"

it("maps / to all", () => {
  let path = "/"
  let expected = AnalyticsType.Grid

  let actual = pathToTag(path)
  expect(actual).toBe(expected)
})

it("maps something weird to unknown", () => {
  let path = "/woah/there"
  let expected = AnalyticsType.Unknown

  let actual = pathToTag(path)
  expect(actual).toBe(expected)
})

it("maps grid", () => {
  expect(pathToTag("/overview")).toBe(AnalyticsType.Grid)
})

it("maps resource detail", () => {
  expect(pathToTag("/r/(all)/overview")).toBe(AnalyticsType.Detail)
})

it("maps filters", () => {
  let loc = {
    pathname: "/r/vigoda/overview",
    search: "?level=error&source=build",
  }
  expect(navigationToTags(loc as Location<{ action: Action }>, "PUSH")).toEqual(
    {
      level: "error",
      source: "build",
      type: AnalyticsType.Detail,
    }
  )
})
