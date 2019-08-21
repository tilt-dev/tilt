import React from "react"
import renderer from "react-test-renderer"
import ShareSnapshotModal from "./ShareSnapshotModal"
import { Snapshot } from "./types"

const testState: Snapshot = {
  Message: "",
  View: null,
  IsSidebarClosed: false,
}
const fakeSendsnapshot = (snapshot: Snapshot) => void {}
const fakeHandleCloseModal = () => {}

describe("ShareSnapshotModal", () => {
  it("renders with modal open", () => {
    const tree = renderer
      .create(
        <ShareSnapshotModal
          handleSendSnapshot={fakeSendsnapshot}
          handleClose={fakeHandleCloseModal}
          show={true}
          state={testState}
          snapshotUrl="http://test.com"
          registerTokenUrl="http://registertoken.com"
        />
      )
      .toJSON()

    expect(tree).toMatchSnapshot()
  })

  it("renders with modal closed", () => {
    const tree = renderer
      .create(
        <ShareSnapshotModal
          handleSendSnapshot={fakeSendsnapshot}
          handleClose={fakeHandleCloseModal}
          show={false}
          state={testState}
          snapshotUrl="http://test.com"
          registerTokenUrl="http://registertoken.com"
        />
      )
      .toJSON()

    expect(tree).toMatchSnapshot()
  })
})
