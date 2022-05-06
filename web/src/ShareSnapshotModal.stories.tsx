import React from "react"
import ReactModal from "react-modal"
import ShareSnapshotModal from "./ShareSnapshotModal"

ReactModal.setAppElement("#root")

let handleSendSnapshot = () => console.log("sendSnapshot")
let handleClose = () => console.log("close")

let signedOut = () => {
  return (
    <ShareSnapshotModal
      handleSendSnapshot={handleSendSnapshot}
      getSnapshot={() => {
        return {}
      }}
      handleClose={handleClose}
      snapshotUrl={""}
      tiltCloudUsername={""}
      tiltCloudSchemeHost={"https://cloud.tilt.dev"}
      tiltCloudTeamID={""}
      isOpen={true}
      highlightedLines={null}
      dialogAnchor={null}
    />
  )
}

let signedIn = () => {
  return (
    <ShareSnapshotModal
      handleSendSnapshot={handleSendSnapshot}
      getSnapshot={() => {
        return {}
      }}
      handleClose={handleClose}
      snapshotUrl={""}
      tiltCloudUsername={"peridot"}
      tiltCloudSchemeHost={"https://cloud.tilt.dev"}
      tiltCloudTeamID={""}
      isOpen={true}
      highlightedLines={null}
      dialogAnchor={null}
    />
  )
}

let withUrl = () => {
  return (
    <ShareSnapshotModal
      handleSendSnapshot={handleSendSnapshot}
      getSnapshot={() => {
        return {}
      }}
      handleClose={handleClose}
      snapshotUrl={"https://cloud.tilt.dev/snapshot/garnet"}
      tiltCloudUsername={"peridot"}
      tiltCloudSchemeHost={"https://cloud.tilt.dev"}
      tiltCloudTeamID={""}
      isOpen={true}
      highlightedLines={null}
      dialogAnchor={null}
    />
  )
}

let withUrlOverflow = () => {
  return (
    <ShareSnapshotModal
      handleSendSnapshot={handleSendSnapshot}
      getSnapshot={() => {
        return {}
      }}
      handleClose={handleClose}
      snapshotUrl={
        "https://cloud.tilt.dev/snapshot/rose-quartz-long-overflow-string"
      }
      tiltCloudUsername={"peridot"}
      tiltCloudSchemeHost={"https://cloud.tilt.dev"}
      tiltCloudTeamID={""}
      isOpen={true}
      highlightedLines={null}
      dialogAnchor={null}
    />
  )
}

let withTeam = () => {
  return (
    <ShareSnapshotModal
      handleSendSnapshot={handleSendSnapshot}
      getSnapshot={() => {
        return {}
      }}
      handleClose={handleClose}
      snapshotUrl={""}
      tiltCloudUsername={"peridot"}
      tiltCloudSchemeHost={"https://cloud.tilt.dev"}
      tiltCloudTeamID={"3e8e3af3-52e7-4f86-9006-9b1cce9ec85d"}
      isOpen={true}
      highlightedLines={null}
      dialogAnchor={null}
    />
  )
}

export default {
  title: "New UI/Shared/ShareSnapshotModal",
}

export const SignedOut = signedOut

export const SignedIn = signedIn

export const WithUrl = withUrl

export const WithUrlOverflow = withUrlOverflow

export const WithTeam = withTeam
