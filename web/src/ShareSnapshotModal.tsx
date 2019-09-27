import React, { PureComponent } from "react"
import Modal from "react-modal"
import "./ShareSnapshotModal.scss"
import cookies from "js-cookie"
import intro from "./assets/png/share-snapshot-intro.png"

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
    return (
      <Modal
        onRequestClose={this.props.handleClose}
        isOpen={this.props.isOpen}
        className="ShareSnapshotModal"
      >
        <h2 className="ShareSnapshotModal-title">Share a Shapshot</h2>
        <div className="ShareSnapshotModal-pane u-flexColumn">
          <div className="ShareSnapshotModal-description">
            Get a link to a snapshot — a browsable, sharable view of the current
            state of your Tilt session.
          </div>
          {this.renderCallToAction()}
        </div>
        {this.maybeRenderManageSnapshots()}
      </Modal>
    )
  }

  renderCallToAction() {
    if (this.props.tiltCloudUsername) {
      return (
        <div>
          {this.renderShareLink()}
          {this.renderLoginState()}
        </div>
      )
    }
    return (
      <div>
        {this.renderIntro()}
        {this.renderGetStarted()}
      </div>
    )
  }

  renderIntro() {
    return (
      <div className="ShareSnapshotModal-intro">
        <div className="u-inlineBlock">
          <img src={intro} className="ShareSnapshotModal-introImage" />
        </div>
        <div className="u-inlineBlock">
          <ul className="ShareSnapshotModal-examples">
            <li>Share errors easily</li>
            <li>Explore logs in-context</li>
            <li>Work together to figure out the problem</li>
          </ul>
        </div>
      </div>
    )
  }

  renderGetStarted() {
    return (
      <div className="ShareSnapshotModal-getStarted">
        <div className="u-inlineBlock">Link Tilt to TiltCloud</div>
        <form
          action={this.props.tiltCloudSchemeHost + "/start_register_token"}
          target="_blank"
          method="POST"
          onSubmit={ShareSnapshotModal.notifyTiltOfRegistration}
        >
          <input name="token" type="hidden" value={cookies.get("Tilt-Token")} />
          <input
            type="submit"
            className="ShareSnapshotModal-button ShareSnapshotModal-button--cta"
            value="Get Started"
          />
        </form>
      </div>
    )
  }

  renderShareLink() {
    const hasLink = this.props.snapshotUrl !== ""
    return (
      <div className="ShareSnapshotModal-shareLink">
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
        {this.renderGetLinkButton()}
      </div>
    )
  }

  renderGetLinkButton() {
    const hasLink = this.props.snapshotUrl !== ""
    if (hasLink) {
      return (
        <a
          className="ShareSnapshotModal-button ShareSnapshotModal-button--inline"
          href={this.props.snapshotUrl}
          target="_blank"
        >
          Open
        </a>
      )
    }
    return (
      <button
        className="ShareSnapshotModal-button ShareSnapshotModal-button--inline"
        onClick={this.props.handleSendSnapshot}
      >
        Get Link
      </button>
    )
  }

  renderLoginState() {
    return (
      <div className="ShareSnapshotModal-loginState">
        Sharing as <strong>{this.props.tiltCloudUsername}</strong>
      </div>
    )
  }

  maybeRenderManageSnapshots() {
    if (!this.props.tiltCloudUsername) {
      return null
    }
    return (
      <div className="ShareSnapshotModal-pane">
        <div className="ShareSnapshotModal-description">
          Manage your snapshots on{" "}
          <a
            href={this.props.tiltCloudSchemeHost + "/snapshots"}
            target="_blank"
          >
            TiltCloud
          </a>{" "}
          →
        </div>
      </div>
    )
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
