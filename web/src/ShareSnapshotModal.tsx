import React, { PureComponent } from "react"
import Modal from "react-modal"
import { Snapshot } from "./types"
import "./ShareSnapshotModal.scss"

type props = {
  handleSendSnapshot: () => void
  handleClose: () => void
  snapshotUrl: string
  registerTokenUrl: string
  isOpen: boolean
}

// TODO(dmiller): in the future this should also show if you are logged in and who you are
export default class ShareSnapshotModal extends PureComponent<props> {
  render() {
    let link = this.renderShareLink()
    return (
      <Modal
        onRequestClose={this.props.handleClose}
        isOpen={this.props.isOpen}
        className="ShareSnapshotModal"
      >
        <div className="ShareSnapshotModal-pane">{link}</div>
        <div className="ShareSnapshotModal-pane">
          {this.loggedOutTiltCloudCopy()}
        </div>
      </Modal>
    )
  }

  renderShareLink() {
    const hasLink = this.props.snapshotUrl !== ""
    if (!hasLink) {
      return (
        <button onClick={this.props.handleSendSnapshot}>Share Snapshot</button>
      )
    }
    return (
      <a href={this.props.snapshotUrl} target="_blank">
        Open link in new tab
      </a>
    )
  }

  loggedOutTiltCloudCopy() {
    return (
      <div>
        <div>
          Go to <a href={this.props.registerTokenUrl}>TiltCloud</a> to manage
          your snapshots.
        </div>
        <div>(You'll just need to link your GitHub Account)</div>
      </div>
    )
  }
}
