import React from "react"
import renderer from "react-test-renderer"
import ShareSnapshotModal from "./ShareSnapshotModal"

const fakeSendsnapshot = () => {}
const fakeHandleCloseModal = () => {}

describe("ShareSnapshotModal", () => {
  it("renders with modal open", () => {
    const tree = renderer
      .create(
        <ShareSnapshotModal
          handleSendSnapshot={fakeSendsnapshot}
          handleClose={fakeHandleCloseModal}
          show={true}
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
          snapshotUrl="http://test.com"
          registerTokenUrl="http://registertoken.com"
        />
      )
      .toJSON()

    expect(tree).toMatchSnapshot()
  })
})
