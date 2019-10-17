import React, { PureComponent } from "react"
import Modal from "react-modal"
import "./ShareSnapshotModal.scss"
import cookies from "js-cookie"
import intro from "./assets/png/share-snapshot-intro.png"
import { ReactComponent as ArrowSvg } from "./assets/svg/arrow.svg"

type props = {
  handleSendSnapshot: () => void
  handleClose: () => void
  snapshotUrl: string
  tiltCloudUsername: string | null
  tiltCloudSchemeHost: string
  tiltCloudTeamID: string | null
  isOpen: boolean
  highlightedLines: number | null
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
        <section className="ShareSnapshotModal-pane u-flexColumn">
          <p className="ShareSnapshotModal-description">
            Get a link to a snapshot — a browsable, sharable view of the current
            state of your Tilt session.
          </p>
          {this.renderCallToAction()}
        </section>
        {this.maybeRenderManageSnapshots()}
      </Modal>
    )
  }

  renderCallToAction() {
    if (this.props.tiltCloudUsername) {
      return (
        <section className="ShareSnapshotModal-shareLinkWrap">
          {this.renderShareLink()}
          {this.renderShareLinkInfo()}
        </section>
      )
    }

    return (
      <section>
        {this.renderIntro()}
        {this.renderGetStarted()}
      </section>
    )
  }

  renderIntro() {
    return (
      <div className="ShareSnapshotModal-intro">
        <div className="u-inlineBlock">
          <img
            src={intro}
            className="ShareSnapshotModal-introImage"
            alt="hand holding up a copy of the Tilt User Interface"
          />
        </div>
        <div className="u-inlineBlock ShareSnapshotModal-details">
          <ul className="ShareSnapshotModal-detailsList">
            <li>Share errors easily</li>
            <li>Explore logs in-context</li>
            <li>Work together to figure out the problem</li>
          </ul>
          <a
            className="ShareSnapshotModal-docsLink"
            href="https://docs.tilt.dev/snapshots.html"
            target="_blank"
            rel="noopener noreferrer"
          >
            Learn more in our docs
          </a>
        </div>
      </div>
    )
  }

  renderGetStarted() {
    return (
      <div className="ShareSnapshotModal-getStarted">
        <p className="u-inlineBlock">
          Link Tilt to TiltCloud (just takes a minute)
        </p>
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
          rel="noopener noreferrer"
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

  renderShareLinkInfo() {
    let lines = this.props.highlightedLines
    return (
      <section className="ShareSnapshotModal-shareLinkInfo">
        <p className="ShareSnapshotModal-loginState">
          Sharing as <strong>{this.props.tiltCloudUsername}</strong>
        </p>
        {lines && (
          <p>
            {lines} Line{lines > 1 ? "s" : ""} Highlighted
          </p>
        )}
      </section>
    )
  }

  maybeRenderManageSnapshots() {
    if (!this.props.tiltCloudUsername) {
      return null
    }
    return (
      <section className="ShareSnapshotModal-manageSnapshots">
        <p>
          Manage your snapshots on{" "}
          <a
            className="ShareSnapshotModal-tiltCloudLink"
            href={this.props.tiltCloudSchemeHost + "/snapshots"}
            target="_blank"
            rel="noopener noreferrer"
          >
            <span>TiltCloud</span>
            <ArrowSvg />
          </a>
        </p>
        {this.props.tiltCloudTeamID ? (
          <p>
            View snapshots from your{" "}
            <a
              className="ShareSnapshotModal-tiltCloudLink"
              rel="noopener noreferrer"
              href={`${this.props.tiltCloudSchemeHost}/team/${this.props.tiltCloudTeamID}/snapshots`}
              target="_blank"
            >
              <span>team</span>
              <ArrowSvg />
            </a>
          </p>
        ) : null}
      </section>
    )
  }

  static notifyTiltOfRegistration() {
    let url = `${window.location.protocol}//${window.location.host}/api/user_started_tilt_cloud_registration`
    fetch(url, {
      method: "POST",
      headers: {
        Accept: "application/json",
        "Content-Type": "application/json",
      },
    })
  }
}
