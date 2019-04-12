import { pathToTag } from "./analytics"

it("maps logs to logs", () => {
  let path = "/r/vigoda"
  let expected = "log"

  let actual = pathToTag(path)
  expect(actual).toBe(expected)
})

it("maps previews to preview", () => {
  let path = "/r/vigoda/preview"
  let expected = "preview"

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
