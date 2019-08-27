import { Snapshot } from "./types"
import * as _ from "lodash"

export default function cleanStateForSnapshotPOST(state: Snapshot): Snapshot {
  let snapshot = _.cloneDeep(state)

  snapshot.SnapshotLink = ""
  snapshot.showSnapshotModal = false
  if (snapshot.View) {
    snapshot.View.FeatureFlags["snapshots"] = false
  }

  return snapshot
}
