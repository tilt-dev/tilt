import { CameraAlt } from "@material-ui/icons"
import { saveAs } from "file-saver"
import moment from "moment"
import React from "react"
import styled from "styled-components"
import { linkToTiltDocs, TiltDocsPage } from "./constants"
import FloatDialog from "./FloatDialog"
import { InstrumentedButton } from "./instrumentedComponents"
import "./ShareSnapshotModal.scss"
import { Color, Font, FontSize, SizeUnit } from "./style-helpers"

type ShareSnapshotModalProps = {
  getSnapshot: () => Proto.webviewSnapshot
  handleClose: () => void
  isOpen: boolean
  dialogAnchor: HTMLElement | null
}

// TODO (lizz): When offline snapshots are the default method for snapshots
// generation, cloud snapshot code can be refactored and this component renamed
export default function ShareSnapshotModal(props: ShareSnapshotModalProps) {
  return (
    <LocalSnapshotDialog
      dialogAnchor={props.dialogAnchor}
      handleClose={props.handleClose}
      isOpen={props.isOpen}
      getSnapshot={props.getSnapshot}
    />
  )
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

export function formatTimestampForFilename(date: moment.Moment) {
  return date.format("YYYY-MM-DD_HHmmss")
}

function downloadSnapshot(snapshot: Proto.webviewSnapshot) {
  const timestamp = snapshot.createdAt ? moment(snapshot.createdAt) : moment()
  const data = new Blob([JSON.stringify(snapshot)], {
    type: "application/json",
  })

  saveAs(data, `tilt-snapshot_${formatTimestampForFilename(timestamp)}.json`)
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
        onClick={() => downloadSnapshot(getSnapshot())}
        startIcon={<CameraAlt />}
      >
        Save Snapshot
      </SaveSnapshotButton>
      <p>
        View your saved snapshot with:{" "}
        <CodeSnippet>tilt snapshot view {"<filename>"}</CodeSnippet>.
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
