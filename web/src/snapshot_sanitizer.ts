import { Snapshot } from "./types"

export default function cleanStateForSnapshotPOST(state: Snapshot): Snapshot {
  let snapshot = Object.assign({}, state)
  snapshot.SnapshotLink = ""
  snapshot.showSnapshotModal = false
  if (snapshot.View) {
    snapshot.View.FeatureFlags["snapshots"] = false
  }
  return snapshot
}
