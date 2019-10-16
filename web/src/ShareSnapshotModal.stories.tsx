import React from "react"
import { storiesOf } from "@storybook/react"

import ShareSnapshotModal from "./ShareSnapshotModal"
import ReactModal from "react-modal"

ReactModal.setAppElement(document.body)

let handleSendSnapshot = () => console.log("sendSnapshot")
let handleClose = () => console.log("close")

let signedOut = () => {
  return (
    <ShareSnapshotModal
      handleSendSnapshot={handleSendSnapshot}
      handleClose={handleClose}
      snapshotUrl={""}
      tiltCloudUsername={""}
      tiltCloudSchemeHost={"https://cloud.tilt.dev"}
      tiltCloudTeamID={""}
      isOpen={true}
      highlightedLines={null}
    />
  )
}

let signedIn = () => {
  return (
    <ShareSnapshotModal
      handleSendSnapshot={handleSendSnapshot}
      handleClose={handleClose}
      snapshotUrl={""}
      tiltCloudUsername={"peridot"}
      tiltCloudSchemeHost={"https://cloud.tilt.dev"}
      tiltCloudTeamID={""}
      isOpen={true}
      highlightedLines={null}
    />
  )
}

let withUrl = () => {
  return (
    <ShareSnapshotModal
      handleSendSnapshot={handleSendSnapshot}
      handleClose={handleClose}
      snapshotUrl={"https://cloud.tilt.dev/snapshot/garnet"}
      tiltCloudUsername={"peridot"}
      tiltCloudSchemeHost={"https://cloud.tilt.dev"}
      tiltCloudTeamID={""}
      isOpen={true}
      highlightedLines={null}
    />
  )
}

let withUrlOverflow = () => {
  return (
    <ShareSnapshotModal
      handleSendSnapshot={handleSendSnapshot}
      handleClose={handleClose}
      snapshotUrl={
        "https://cloud.tilt.dev/snapshot/rose-quartz-long-overflow-string"
      }
      tiltCloudUsername={"peridot"}
      tiltCloudSchemeHost={"https://cloud.tilt.dev"}
      tiltCloudTeamID={""}
      isOpen={true}
      highlightedLines={null}
    />
  )
}

let withTeam = () => {
  return (
    <ShareSnapshotModal
      handleSendSnapshot={handleSendSnapshot}
      handleClose={handleClose}
      snapshotUrl={""}
      tiltCloudUsername={"peridot"}
      tiltCloudSchemeHost={"https://cloud.tilt.dev"}
      tiltCloudTeamID={"3e8e3af3-52e7-4f86-9006-9b1cce9ec85d"}
      isOpen={true}
      highlightedLines={null}
    />
  )
}

storiesOf("ShareSnapshotModal", module)
  .add("signed-out", signedOut)
  .add("signed-in", signedIn)
  .add("with-url", withUrl)
  .add("with-url-overflow", withUrlOverflow)
  .add("with-team", withTeam)
