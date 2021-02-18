import { navigationToTags, pathToTag } from "./analytics"

it("maps / to all", () => {
  let path = "/"
  let expected = "grid"

  let actual = pathToTag(path)
  expect(actual).toBe(expected)
})

it("maps something weird to unknown", () => {
  let path = "/woah/there"
  let expected = "unknown"

  let actual = pathToTag(path)
  expect(actual).toBe(expected)
})

it("maps grid", () => {
  expect(pathToTag("/overview")).toBe("grid")
})

it("maps resource detail", () => {
  expect(pathToTag("/r/(all)/overview")).toBe("resource-detail")
})

it("maps filters", () => {
  let loc = {
    pathname: "/r/vigoda/overview",
    search: "?level=error&source=build",
  }
  expect(navigationToTags(loc, "PUSH")).toEqual({
    level: "error",
    source: "build",
    type: "resource-detail",
  })
})
