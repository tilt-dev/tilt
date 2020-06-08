import React, { PureComponent } from "react"
import Modal from "react-modal"
import { ShowErrorModal } from "./types"
import "./FatalErrorModal.scss"

type props = {
  error: string | undefined
  showErrorModal: ShowErrorModal
  handleClose: () => void
}

export default class ErrorModal extends PureComponent<props> {
  render() {
    let showModal =
      Boolean(this.props.error) &&
      (this.props.showErrorModal === ShowErrorModal.Default ||
        this.props.showErrorModal === ShowErrorModal.Show)
    return (
      <Modal
        isOpen={showModal}
        className="FatalErrorModal"
        onRequestClose={this.props.handleClose}
      >
        <h2 className="FatalErrorModal-title">Error</h2>
        <div className="FatalErrorModal-pane">
          <p>Tilt has encountered an error: {this.props.error}</p>
        </div>
        <div className="FatalErrorModal-pane">
          <p>
            To get help try{" "}
            <a href="https://github.com/tilt-dev/tilt/issues/new">
              opening a GitHub issue
            </a>{" "}
            or{" "}
            <a href="https://docs.tilt.dev/debug_faq.html#where-can-i-ask-questions">
              contacting us on Slack
            </a>
            .
          </p>
        </div>
      </Modal>
    )
  }
}
