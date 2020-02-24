import { pathToTag } from "./analytics"

it("maps logs to logs", () => {
  let path = "/r/vigoda"
  let expected = "log"

  let actual = pathToTag(path)
  expect(actual).toBe(expected)
})

it("maps / to all", () => {
  let path = "/"
  let expected = "all"

  let actual = pathToTag(path)
  expect(actual).toBe(expected)
})

it("maps something weird to unknown", () => {
  let path = "/woah/there"
  let expected = "unknown"

  let actual = pathToTag(path)
  expect(actual).toBe(expected)
})

it("maps errors to errors", () => {
  let path = "/alerts"
  let expected = "errors"

  let actual = pathToTag(path)
  expect(actual).toBe(expected)

  path = "/r/foo/alerts"
  actual = pathToTag(path)
  expect(actual).toBe(expected)
})

it("maps trace", () => {
  expect(pathToTag("/r/fe/trace/build:1")).toBe("trace")
})
