import { render } from "@testing-library/react"
import React from "react"
import { FeaturesProvider, Flag } from "./feature"
import PathBuilder, { PathBuilderProvider } from "./PathBuilder"
import { SnapshotActionProvider, useSnapshotAction } from "./snapshot"

// Make sure that useSnapshotAction() doesn't break memoization.
it("memoizes renders", () => {
  let renderCount = 0
  let FakeEl = React.memo(() => {
    useSnapshotAction()
    renderCount++
    return <div></div>
  })

  let pathBuilder = PathBuilder.forTesting("localhost", "/")
  let openModal = () => {}
  let tree = (flags: Proto.v1alpha1UIFeatureFlag[]) => {
    return (
      <FeaturesProvider featureFlags={flags}>
        <PathBuilderProvider value={pathBuilder}>
          <SnapshotActionProvider openModal={openModal}>
            <FakeEl />
          </SnapshotActionProvider>
        </PathBuilderProvider>
      </FeaturesProvider>
    )
  }

  let flags = [{ name: "foo", value: true }]
  let { rerender } = render(tree(flags))
  expect(renderCount).toEqual(1)

  // Make sure we don't re-render if an irrelevant flag changes
  let flags2 = [{ name: "foo", value: false }]
  rerender(tree(flags2))
  expect(renderCount).toEqual(1)

  // Make sure we do re-render on a real update.
  let flags3 = [{ name: Flag.Snapshots, value: true }]
  rerender(tree(flags3))
  expect(renderCount).toEqual(2)
})
