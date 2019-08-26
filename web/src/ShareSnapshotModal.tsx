import React, { PureComponent } from "react"
import Modal from "react-modal"
import "./ShareSnapshotModal.scss"
import cookies from "js-cookie"

type props = {
  handleSendSnapshot: () => void
  handleClose: () => void
  snapshotUrl: string
  tiltCloudUsername: string
  tiltCloudSchemeHost: string
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
          {this.tiltCloudCopy()}
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

  tiltCloudCopy() {
    if (this.props.tiltCloudUsername == "") {
      return (
          <div>
            <div>
              Click <form action={this.props.tiltCloudSchemeHost + "/register_token"} target="_blank" method="POST">
              <input type="submit" value="here"/>
              <input name="token" type="hidden" value={cookies.get("Tilt-Token")}/>
            </form>
              to associate this copy of Tilt with your TiltCloud account.
            </div>
            <div>(You'll just need to link your GitHub Account)</div>
          </div>
      )
    } else {
      return (
          <div>
            Sharing snapshots as {this.props.tiltCloudUsername}.
          </div>
      )
    }
  }
}
