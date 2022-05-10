import { CameraAlt } from "@material-ui/icons"
import { saveAs } from "file-saver"
import cookies from "js-cookie"
import moment from "moment"
import React, { PureComponent } from "react"
import Modal from "react-modal"
import styled from "styled-components"
import intro from "./assets/png/share-snapshot-intro.png"
import { ReactComponent as ArrowSvg } from "./assets/svg/arrow.svg"
import { linkToTiltDocs, TiltDocsPage } from "./constants"
import { Flag, useFeatures } from "./feature"
import FloatDialog from "./FloatDialog"
import { InstrumentedButton } from "./instrumentedComponents"
import "./ShareSnapshotModal.scss"
import { Color, Font, FontSize, SizeUnit } from "./style-helpers"

type ShareSnapshotModalProps = {
  handleSendSnapshot: (s: Proto.webviewSnapshot) => void
  getSnapshot: () => Proto.webviewSnapshot
  handleClose: () => void
  snapshotUrl: string
  tiltCloudUsername: string | null
  tiltCloudSchemeHost: string
  tiltCloudTeamID: string | null
  isOpen: boolean
  highlightedLines: number | null
  dialogAnchor: HTMLElement | null
}

// TODO (lizz): When offline snapshots are the default method for snapshots
// generation, cloud snapshot code can be refactored and this component renamed
export default function ShareSnapshotModal(props: ShareSnapshotModalProps) {
  const features = useFeatures()
  if (!features.isEnabled(Flag.OfflineSnapshotCreation)) {
    return <CloudSnapshotModal {...props} />
  } else {
    return (
      <LocalSnapshotDialog
        dialogAnchor={props.dialogAnchor}
        handleClose={props.handleClose}
        isOpen={props.isOpen}
        getSnapshot={props.getSnapshot}
      />
    )
  }
}

class CloudSnapshotModal extends PureComponent<ShareSnapshotModalProps> {
  render() {
    return (
      <Modal
        onRequestClose={this.props.handleClose}
        isOpen={this.props.isOpen}
        className="ShareSnapshotModal"
      >
        <h2 className="ShareSnapshotModal-title">Share a Snapshot</h2>
        <section className="ShareSnapshotModal-pane u-flexColumn">
          <p className="ShareSnapshotModal-description">
            Get a link to a{" "}
            {this.props.tiltCloudTeamID ? "private team " : null} snapshot — a
            browsable, sharable view of the current state of your Tilt session.
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
          Link Tilt to Tilt Cloud (just takes a minute)
        </p>
        <form
          action={this.props.tiltCloudSchemeHost + "/start_register_token"}
          target="_blank"
          method="POST"
          onSubmit={CloudSnapshotModal.notifyTiltOfRegistration}
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
        onClick={() => this.props.handleSendSnapshot(this.props.getSnapshot())}
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
            <span>Tilt Cloud</span>
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

type DownloadSnapshotModalProps = {
  handleClose: () => void
  getSnapshot: () => Proto.webviewSnapshot
  isOpen: boolean
  dialogAnchor: HTMLElement | null
}

const SaveSnapshotButton = styled(InstrumentedButton)`
  background-color: ${Color.gray70};
  border: 1px solid ${Color.gray10};
  color: ${Color.gray10};
  display: flex;
  font-family: ${Font.monospace};
  font-size: ${FontSize.default};
  margin: ${SizeUnit(0.75)} 0;
  text-transform: unset;

  &:focus,
  &:active {
    background-color: rgba(0, 0, 0, 0.04);
  }
`

const CodeSnippet = styled.code`
  background-color: ${Color.offWhite};
  padding: 0 5px;
  white-space: nowrap;
`

function downloadSnapshot(snapshot: Proto.webviewSnapshot) {
  const timestamp = moment().format("YYYY-MM-DD_HHmmss")
  const data = new Blob([JSON.stringify(snapshot)], {
    type: "application/json",
  })

  saveAs(data, `tilt-snapshot_${timestamp}.json`)
}

export function LocalSnapshotDialog(props: DownloadSnapshotModalProps) {
  const { handleClose, getSnapshot, isOpen, dialogAnchor } = props
  return (
    <FloatDialog
      id="download-snapshot"
      title="Create a Snapshot"
      open={isOpen}
      onClose={handleClose}
      anchorEl={dialogAnchor}
    >
      <p>
        Snapshots let you save and share a browsable view of your Tilt session.
        You can download a snapshot here or with:{" "}
        <CodeSnippet>tilt snapshot create</CodeSnippet>.
      </p>

      <SaveSnapshotButton
        analyticsName="ui.web.downloadSnapshot"
        onClick={() => downloadSnapshot(getSnapshot())}
        startIcon={<CameraAlt />}
      >
        Save Snapshot
      </SaveSnapshotButton>
      <p>
        View your saved snapshot with:{" "}
        <CodeSnippet>tilt view snapshot {"<filename>"}</CodeSnippet>.
      </p>
      <p>
        See the{" "}
        <a
          href={linkToTiltDocs(TiltDocsPage.Snapshots)}
          target="_blank"
          rel="noopener noreferrer"
        >
          snapshot docs
        </a>{" "}
        for more info.
      </p>
    </FloatDialog>
  )
}
