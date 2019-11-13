import { JSDOM } from "jsdom"
import { logPaneDOM } from "./testdata"
import findLogLineID from "./findLogLine"

describe("findLogLine", () => {
  it("returns null if passed null", () => {
    const actual = findLogLineID(null)
    expect(actual).toBeNull()
  })

  it("returns the value of data-lineid if the element has data-lineid", () => {
    const dom = new JSDOM(logPaneDOM)
    const node = dom.window.document.getElementById("start1")

    const actual = findLogLineID(node)
    expect(actual).toBe("1920")
  })

  it("returns the value of parent's data-lineid if the element has no data-lineid", () => {
    const dom = new JSDOM(logPaneDOM)
    const node = dom.window.document.getElementById("start2")

    const actual = findLogLineID(node)
    expect(actual).toBe("1920")
  })

  // TODO(dmiller): how to test the last case, that there's a Node that is passed in instead of an HTMLElement
})
