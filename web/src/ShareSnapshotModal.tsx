import React, { PureComponent } from "react"
import Modal from "react-modal"
import "./ShareSnapshotModal.scss"
import cookies from "js-cookie"

type props = {
  handleSendSnapshot: () => void
  handleClose: () => void
  snapshotUrl: string
  tiltCloudUsername: string | null
  tiltCloudSchemeHost: string
  isOpen: boolean
}

export default class ShareSnapshotModal extends PureComponent<props> {
  render() {
    let tcUser = this.props.tiltCloudUsername;
    let user = tcUser ? tcUser : "[anonymous user]";
    let link = this.renderShareLink();
    return (
      <Modal
        onRequestClose={this.props.handleClose}
        isOpen={this.props.isOpen}
        className="ShareSnapshotModal"
      >
        <h2 className="ShareSnapshotModal-title">Share a Shapshot</h2>
        <div className="ShareSnapshotModal-pane">
          <p>Let anyone explore your current Tilt session with an interactive snapshot.</p>
          {link}
          <p className="ShareSnapshotModal-user">Sharing as <strong>{user}</strong></p>
        </div>
        <div className="ShareSnapshotModal-pane">
          {this.tiltCloudCopy()}
        </div>
      </Modal>
    )
  }

  renderShareLink() {
    const hasLink = this.props.snapshotUrl !== "";
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
    if (!this.props.tiltCloudUsername) {
      return (
        <section className="ShareSnapshotModal-tiltCloudCopy">
          <p>Register on TiltCloud to share under your name, view, and delete snapshots.</p>
          <form
            className="ShareSnapshotModal-tiltCloudButtonForm"
            action={this.props.tiltCloudSchemeHost + "/register_token"}
            target="_blank"
            method="POST"
            onSubmit={ShareSnapshotModal.notifyTiltOfRegistration}
          >
            <p>You'll need a GitHub account</p>
            <span className="ShareSnapshotModal-tiltCloudButtonForm-spacer" />
            <input className="ShareSnapshotModal-tiltCloudButton" type="submit" value="Sign Up" />
            <input
              name="token"
              type="hidden"
              value={cookies.get("Tilt-Token")}
            />
          </form>
        </section>
      )
    } else {
      return (
        <p>View and delete snapshots on <a href="http://cloud.tilt.dev/snapshots" target="_blank">TiltCloud</a></p>
      )
    }
  }

  static notifyTiltOfRegistration() {
    let url = `${window.location.protocol}//${
      window.location.host
    }/api/user_started_tilt_cloud_registration`
    fetch(url, {
      method: "POST",
      headers: {
        Accept: "application/json",
        "Content-Type": "application/json",
      },
    })
  }
}
