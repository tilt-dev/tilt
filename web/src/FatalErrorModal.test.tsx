import React from "react"
import ReactDOM from "react-dom"
import ReactModal from "react-modal"
import renderer from "react-test-renderer"
import FatalErrorModal from "./FatalErrorModal"
import { ShowFatalErrorModal } from "./types"

const fakeHandleCloseModal = () => {}
let originalCreatePortal = ReactDOM.createPortal

describe("FatalErrorModal", () => {
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

  it("doesn't render if there's no fatal error and the modal hasn't been closed", () => {
    const tree = renderer
      .create(
        <FatalErrorModal
          error={null}
          showFatalErrorModal={ShowFatalErrorModal.Default}
          handleClose={fakeHandleCloseModal}
        />
      )
      .toJSON()

    expect(tree).toMatchSnapshot()
  })

  it("does render if there is a fatal error and the modal hasn't been closed", () => {
    const tree = renderer
      .create(
        <FatalErrorModal
          error={"i'm an error"}
          showFatalErrorModal={ShowFatalErrorModal.Default}
          handleClose={fakeHandleCloseModal}
        />
      )
      .toJSON()

    expect(tree).toMatchSnapshot()
  })

  it("doesn't render if there is a fatal error and the modal has been closed", () => {
    const tree = renderer
      .create(
        <FatalErrorModal
          error={"i'm an error"}
          showFatalErrorModal={ShowFatalErrorModal.Hide}
          handleClose={fakeHandleCloseModal}
        />
      )
      .toJSON()

    expect(tree).toMatchSnapshot()
  })
})
