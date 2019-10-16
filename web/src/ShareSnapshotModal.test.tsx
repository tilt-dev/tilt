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

  it("renders with modal open w/o known username", () => {
    const tree = renderer
      .create(
        <ShareSnapshotModal
          handleSendSnapshot={fakeSendsnapshot}
          handleClose={fakeHandleCloseModal}
          isOpen={true}
          snapshotUrl="http://test.com"
          tiltCloudUsername={null}
          tiltCloudSchemeHost={"https://cloud.tilt.dev"}
          tiltCloudTeamID={null}
          highlightedLines={null}
        />
      )
      .toJSON()

    expect(tree).toMatchSnapshot()
  })

  it("renders with modal open w/ known username", () => {
    const tree = renderer
      .create(
        <ShareSnapshotModal
          handleSendSnapshot={fakeSendsnapshot}
          handleClose={fakeHandleCloseModal}
          isOpen={true}
          snapshotUrl="http://test.com"
          tiltCloudUsername={"tacocat"}
          tiltCloudSchemeHost={"https://cloud.tilt.dev"}
          tiltCloudTeamID={null}
          highlightedLines={null}
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
          tiltCloudUsername={null}
          tiltCloudSchemeHost={"https://cloud.tilt.dev"}
          tiltCloudTeamID={null}
          highlightedLines={null}
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
          tiltCloudUsername={null}
          tiltCloudSchemeHost={"https://cloud.tilt.dev"}
          tiltCloudTeamID={null}
          highlightedLines={null}
        />
      )
      .toJSON()

    expect(tree).toMatchSnapshot()
  })

  it("renders team link", () => {
    const tree = renderer
      .create(
        <ShareSnapshotModal
          handleSendSnapshot={fakeSendsnapshot}
          handleClose={fakeHandleCloseModal}
          isOpen={true}
          snapshotUrl="http://test.com"
          tiltCloudUsername={"Hello"}
          tiltCloudSchemeHost={"https://cloud.tilt.dev"}
          tiltCloudTeamID={"abcdefg"}
          highlightedLines={null}
        />
      )
      .toJSON()

    expect(tree).toMatchSnapshot()
  })
})
