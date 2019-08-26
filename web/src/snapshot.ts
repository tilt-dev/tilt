import { Snapshot } from "./types"

export default function cleanStateForSnapshotPOST(
  snapshot: Snapshot
): Snapshot {
  snapshot.SnapshotLink = ""
  snapshot.showSnapshotModal = false
  return snapshot
}
