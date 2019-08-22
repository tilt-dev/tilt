import React from "react"
import { Snapshot } from "./types"
import "./ShareSnapshotModal.scss"

type props = {
  handleSendSnapshot: () => void
  handleClose: () => void
  show: boolean
  snapshotUrl: string
  registerTokenUrl: string
}
export default function ShareSnapshotModal(props: props) {
  const showHideClassName = props.show
    ? "modal display-block"
    : "modal display-none"
  const hasLink = props.snapshotUrl !== ""
  if (!hasLink) {
    return (
      <div className={showHideClassName}>
        <div className="modal-main">
          <section className="modal-snapshotUrlWrap">
            <button onClick={props.handleClose}>close</button>
            <button onClick={() => props.handleSendSnapshot()}>
              Share Snapshot
            </button>
          </section>
          <hr />
          {loggedOutTiltCloudCopy(props.registerTokenUrl)}
        </div>
      </div>
    )
  }

  return (
    <div className={showHideClassName}>
      <div className="modal-main">
        <section className="modal-snapshotUrlWrap">
          <button onClick={props.handleClose}>close</button>
          <button onClick={() => window.open(props.snapshotUrl)}>
            Open link in new tab
          </button>
        </section>
        <hr />
        {loggedOutTiltCloudCopy("")}
      </div>
    </div>
  )
}

const loggedOutTiltCloudCopy = (registerTokenURL: string) => (
  <section className="modal-cloud">
    <p>
      Go to <a href={registerTokenURL}>TiltCloud</a> to manage your snapshots.
      <br />
      (You'll just need to link your GitHub Account)
    </p>
  </section>
)
