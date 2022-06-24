import { render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import FileSaver from "file-saver"
import moment from "moment"
import React from "react"
import ReactDOM from "react-dom"
import ReactModal from "react-modal"
import ShareSnapshotModal from "./ShareSnapshotModal"
import { nResourceView } from "./testdata"

const FAKE_SNAPSHOT: Proto.webviewSnapshot = { view: nResourceView(1) }
const fakeSendsnapshot = () => {}
const fakeHandleCloseModal = () => {}
const fakeGetSnapshot = () => FAKE_SNAPSHOT
let originalCreatePortal = ReactDOM.createPortal

describe("ShareSnapshotModal", () => {
  beforeEach(() => {
    // Note: `body` is used as the app element _only_ in a test env
    // since the app root element isn't available; in prod, it should
    // be set as the app root so that accessibility features are set correctly
    ReactModal.setAppElement(document.body)
    let mock: any = (node: any) => node
    ReactDOM.createPortal = mock
  })

  afterEach(() => {
    ReactDOM.createPortal = originalCreatePortal
  })

  describe("LocalSnapshotDialog", () => {
    let getSnapshotSpy: jest.Mock
    beforeEach(() => {
      Date.now = jest.fn(() => 1652119615751)
      getSnapshotSpy = jest.fn(() => FAKE_SNAPSHOT)
      jest.mock("file-saver")

      render(
        <ShareSnapshotModal
          handleClose={fakeHandleCloseModal}
          getSnapshot={getSnapshotSpy}
          isOpen={true}
          dialogAnchor={document.body}
        />
      )
    })

    afterEach(() => {
      jest.unmock("file-saver")
      jest.useRealTimers()
    })

    it("gets the snapshot data on download", () => {
      userEvent.click(screen.getByText("Save Snapshot"))

      expect(getSnapshotSpy).toHaveBeenCalled()
    })

    it("saves the snapshot data with correct filename on download", async () => {
      userEvent.click(screen.getByText("Save Snapshot"))

      expect(FileSaver.saveAs).toHaveBeenCalledTimes(1)

      const spyCalls = (FileSaver.saveAs as unknown as jest.Mock).mock.calls
      const blob = spyCalls[0][0] as Blob
      const filename = spyCalls[0][1] as string

      // TODO (lizz): Add this case back in. Right now, parsing the blob
      // causes an ECONNREFUSED error, even though the test passes.
      // const blobText = await blob.text()
      // expect(blobText).toEqual(JSON.stringify(FAKE_SNAPSHOT))

      const testTimestamp = moment().format("YYYY-MM-DD_HHmmss")
      expect(filename).toEqual(`tilt-snapshot_${testTimestamp}.json`)
    }, 120000)
  })
})
