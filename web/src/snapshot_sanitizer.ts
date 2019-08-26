import { Snapshot } from "./types"

export default function cleanStateForSnapshotPOST(
  snapshot: Snapshot
): Snapshot {
  snapshot.SnapshotLink = ""
  snapshot.showSnapshotModal = false
  if (snapshot.View) {
    snapshot.View.FeatureFlags["snapshots"] = false
  }
  return snapshot
}
