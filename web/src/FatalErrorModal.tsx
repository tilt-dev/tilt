import React, { PureComponent } from "react"
import Modal from "react-modal"
import { ShowFatalErrorModal } from "./types"
import "./FatalErrorModal.scss"

type props = {
  error: string | null | undefined
  showFatalErrorModal: ShowFatalErrorModal
  handleClose: () => void
}

export default class FatalErrorModal extends PureComponent<props> {
  render() {
    let showModal =
      Boolean(this.props.error) &&
      (this.props.showFatalErrorModal === ShowFatalErrorModal.Default ||
        this.props.showFatalErrorModal === ShowFatalErrorModal.Show)
    return (
      <Modal
        isOpen={showModal}
        className="FatalErrorModal"
        onRequestClose={this.props.handleClose}
      >
        <h2 className="FatalErrorModal-title">Fatal Error</h2>
        <div className="FatalErrorModal-pane">
          <p>Tilt has encountered a fatal error: {this.props.error}</p>
          <p>
            Once you fix this issue you'll need to restart Tilt. In the meantime
            feel free to browse through the UI.
          </p>
        </div>
      </Modal>
    )
  }
}
