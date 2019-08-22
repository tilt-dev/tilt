import React, { PureComponent } from "react"
import { Snapshot } from "./types"
import "./ShareSnapshotModal.scss"

type props = {
  handleSendSnapshot: () => void
  handleClose: () => void
  show: boolean
  snapshotUrl: string
  registerTokenUrl: string
}

// TODO(dmiller): in the future this should also show if you are logged in and who you are
export default class ShareSnapshotModal extends PureComponent<props> {
  render() {
    const showHideClassName = this.props.show
      ? "modal display-block"
      : "modal display-none"
    const hasLink = this.props.snapshotUrl !== ""
    if (!hasLink) {
      return (
        <div className={showHideClassName}>
          <div className="modal-main">
            <section className="modal-snapshotUrlWrap">
              <button onClick={this.props.handleClose}>close</button>
              <button onClick={() => this.props.handleSendSnapshot()}>
                Share Snapshot
              </button>
            </section>
            <hr />
            {this.loggedOutTiltCloudCopy()}
          </div>
        </div>
      )
    }

    return (
      <div className={showHideClassName}>
        <div className="modal-main">
          <section className="modal-snapshotUrlWrap">
            <button onClick={this.props.handleClose}>close</button>
            <a href={this.props.snapshotUrl} target="_blank">
              Open link in new tab
            </a>
          </section>
          <hr />
          {this.loggedOutTiltCloudCopy()}
        </div>
      </div>
    )
  }

  loggedOutTiltCloudCopy() {
    return (
      <section className="modal-cloud">
        <p>
          Go to <a href={this.props.registerTokenUrl}>TiltCloud</a> to manage
          your snapshots.
          <br />
          (You'll just need to link your GitHub Account)
        </p>
      </section>
    )
  }
}
