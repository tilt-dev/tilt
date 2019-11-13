import { JSDOM } from "jsdom"
import { logPaneDOM } from "./testdata"
import findLogLineID from "./findLogLine"

describe("findLogLine", () => {
  it("returns null is passed null", () => {
    const actual = findLogLineID(null)
    expect(actual).toBeNull()
  })
})
