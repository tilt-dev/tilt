import { render } from "@testing-library/react"
import React from "react"
import Features, { FeaturesProvider, Flag, useFeatures } from "./feature"
import type { UIFeatureFlag } from "./core"

describe("feature", () => {
  it("returns false if the feature does not exist", () => {
    let features = new Features({})
    expect(features.isEnabled("foo" as Flag)).toBe(false)
  })

  it("returns false if the feature does exist and is false", () => {
    let features = new Features({ foo: false })
    expect(features.isEnabled("foo" as Flag)).toBe(false)
  })

  it("returns true if the feature does exist and is true", () => {
    let features = new Features({ foo: true })
    expect(features.isEnabled("foo" as Flag)).toBe(true)
  })

  it("still works if null is passed in", () => {
    let features = new Features(null)
    expect(features.isEnabled("foo" as Flag)).toBe(false)
  })
})

// Make sure that useFeatures() doesn't break memoization.
it("memoizes renders", () => {
  let renderCount = 0
  let FakeEl = React.memo(() => {
    useFeatures()
    renderCount++
    return <div></div>
  })

  let flags = [{ name: "foo", value: true }]
  let tree = (flags: UIFeatureFlag[]) => {
    return (
      <FeaturesProvider featureFlags={flags}>
        <FakeEl />
      </FeaturesProvider>
    )
  }

  let { rerender } = render(tree(flags))

  expect(renderCount).toEqual(1)
  rerender(tree(flags))

  // Make sure we don't re-render on a no-op update.
  expect(renderCount).toEqual(1)

  // Make sure we do re-render on a real update.
  let newFlags = [{ name: "foo", value: false }]
  rerender(tree(newFlags))
  expect(renderCount).toEqual(2)
})
