import Features from "./feature"

describe("feature", () => {
  it("returns false if the feature does not exist", () => {
    let features = new Features({})
    expect(features.isEnabled("foo")).toBe(false)
  })

  it("returns false if the feature does exist and is false", () => {
    let features = new Features({ foo: false })
    expect(features.isEnabled("foo")).toBe(false)
  })

  it("returns true if the feature does exist and is true", () => {
    let features = new Features({ foo: true })
    expect(features.isEnabled("foo")).toBe(true)
  })

  it("still works if null is passed in", () => {
    let features = new Features(null)
    expect(features.isEnabled("foo")).toBe(false)
  })
})
