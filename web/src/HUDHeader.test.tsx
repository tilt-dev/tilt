import React from "react"
import renderer from "react-test-renderer"
import { MemoryRouter } from "react-router"
import { ResourceView, SnapshotHighlight } from "./types"
import HUDHeader from "./HUDHeader"

const fakeHandleOpenModal = () => {}

it("shows snapshot url", () => {
  const tree = renderer
    .create(
      <MemoryRouter>
        <HUDHeader
          showSnapshotButton={true}
          handleOpenModal={fakeHandleOpenModal}
          highlight={null}
        />
      </MemoryRouter>
    )
    .toJSON()

  expect(tree).toMatchSnapshot()
})

it("doesn't render snapshot button if it's a snapshot", () => {
  const tree = renderer
    .create(
      <MemoryRouter>
        <HUDHeader showSnapshotButton={false} />
      </MemoryRouter>
    )
    .toJSON()

  expect(tree).toMatchSnapshot()
})
