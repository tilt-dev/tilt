import React from "react"
import ReactModal from "react-modal"
import Features, { FeaturesTestProvider, Flag } from "./feature"
import ShareSnapshotModal from "./ShareSnapshotModal"
import { nResourceView } from "./testdata"

ReactModal.setAppElement("#root")

let handleSendSnapshot = () => console.log("sendSnapshot")
let handleClose = () => console.log("close")

let signedOut = () => {
  return (
    <ShareSnapshotModal
      handleSendSnapshot={handleSendSnapshot}
      getSnapshot={() => ({})}
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
      getSnapshot={() => ({})}
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
      getSnapshot={() => ({})}
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
      getSnapshot={() => ({})}
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

const offlineSnapshot = () => {
  const features = new Features({ [Flag.OfflineSnapshotCreation]: true })
  const anchorEl: HTMLElement | null = document.body.querySelector("#root")
  return (
    <FeaturesTestProvider value={features}>
      <ShareSnapshotModal
        handleSendSnapshot={handleSendSnapshot}
        getSnapshot={() => ({ view: nResourceView(1) })}
        handleClose={handleClose}
        snapshotUrl=""
        tiltCloudUsername=""
        tiltCloudSchemeHost=""
        tiltCloudTeamID=""
        isOpen={true}
        highlightedLines={null}
        dialogAnchor={anchorEl}
      />
    </FeaturesTestProvider>
  )
}

export default {
  title: "New UI/Shared/ShareSnapshotModal",
}

export const CloudSignedOut = signedOut

export const CloudSignedIn = signedIn

export const CloudWithUrl = withUrl

export const CloudWithUrlOverflow = withUrlOverflow

export const CloudWithTeam = withTeam

export const OfflineSnapshot = offlineSnapshot
