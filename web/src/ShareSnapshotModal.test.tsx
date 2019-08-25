import React from "react"
import ReactDOM from "react-dom"
import ReactModal from "react-modal"
import renderer from "react-test-renderer"
import ShareSnapshotModal from "./ShareSnapshotModal"

const fakeSendsnapshot = () => {}
const fakeHandleCloseModal = () => {}
let originalCreatePortal = ReactDOM.createPortal

describe("ShareSnapshotModal", () => {
  beforeEach(() => {
    ReactModal.setAppElement(document.body)
    let mock: any = (node: any) => node
    ReactDOM.createPortal = mock
  })

  afterEach(() => {
    ReactDOM.createPortal = originalCreatePortal
  })

  it("renders with modal open", () => {
    const tree = renderer
      .create(
        <ShareSnapshotModal
          handleSendSnapshot={fakeSendsnapshot}
          handleClose={fakeHandleCloseModal}
          isOpen={true}
          snapshotUrl="http://test.com"
          registerTokenUrl="http://registertoken.com"
        />
      )
      .toJSON()

    expect(tree).toMatchSnapshot()
  })

  it("renders without snapshotUrl", () => {
    const tree = renderer
      .create(
        <ShareSnapshotModal
          handleSendSnapshot={fakeSendsnapshot}
          handleClose={fakeHandleCloseModal}
          isOpen={true}
          snapshotUrl=""
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
          isOpen={false}
          snapshotUrl="http://test.com"
          registerTokenUrl="http://registertoken.com"
        />
      )
      .toJSON()

    expect(tree).toMatchSnapshot()
  })
})
