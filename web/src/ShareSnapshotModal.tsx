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
    let tcUser = this.props.tiltCloudUsername
    let user = tcUser ? tcUser : "[anonymous user]"
    let link = this.renderShareLink()
    return (
      <Modal
        onRequestClose={this.props.handleClose}
        isOpen={this.props.isOpen}
        className="ShareSnapshotModal"
      >
        <h2 className="ShareSnapshotModal-title">Share a Shapshot</h2>
        <div className="ShareSnapshotModal-pane">
          <p>
            Get a link to share a snapshot of the current state of your Tilt
            session.
          </p>
          {link}
          <p className="ShareSnapshotModal-user">
            Sharing as <strong>{user}</strong>
          </p>
        </div>
        <div className="ShareSnapshotModal-pane">{this.tiltCloudCopy()}</div>
      </Modal>
    )
  }

  renderShareLink() {
    const hasLink = this.props.snapshotUrl !== ""
    return (
      <section className="ShareSnapshotModal-shareLink">
        <div className="ShareSnapshotModal-shareLink-urlBox">
          {hasLink ? (
            <p className="ShareSnapshotModal-shareLink-url">
              {this.props.snapshotUrl}
            </p>
          ) : (
            <p className="ShareSnapshotModal-shareLink-placeholder">
              No Link Generated Yet
            </p>
          )}
        </div>
        {!hasLink && (
          <button
            className="ShareSnapshotModal-button"
            onClick={this.props.handleSendSnapshot}
          >
            Get Link
          </button>
        )}
        {hasLink && (
          <a
            className="ShareSnapshotModal-button"
            href={this.props.snapshotUrl}
            target="_blank"
          >
            Open
          </a>
        )}
      </section>
    )
  }

  tiltCloudCopy() {
    if (!this.props.tiltCloudUsername) {
      return (
        <section>
          <p>
            Connect Tilt to TiltCloud to share under your name and manage your
            snapshots.
          </p>
          <form
            className="ShareSnapshotModal-tiltCloudButtonForm"
            action={this.props.tiltCloudSchemeHost + "/register_token"}
            target="_blank"
            method="POST"
            onSubmit={ShareSnapshotModal.notifyTiltOfRegistration}
          >
            <p>You'll need a GitHub account</p>
            <span className="ShareSnapshotModal-tiltCloudButtonForm-spacer" />
            <input
              className="ShareSnapshotModal-tiltCloudButton"
              type="submit"
              value="Connect to TiltCloud"
            />
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
        <p>
          Manage your snapshots on{" "}
          <a href="http://cloud.tilt.dev/snapshots" target="_blank">
            TiltCloud
          </a>
        </p>
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
