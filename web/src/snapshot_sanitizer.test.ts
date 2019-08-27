import cleanStateForSnapshotPOST from "./snapshot_sanitizer"
import { Snapshot } from "./types"
import { oneResource } from "./testdata.test"
import Features from "./feature"

const testSnapshot = {
  Message: "",
  View: {
    Resources: [oneResource()],
    Log: "",
    LogTimestamps: false,
    SailEnabled: false,
    SailURL: "",
    NeedsAnalyticsNudge: false,
    RunningTiltBuild: {
      Version: "",
      Date: "",
      Dev: false,
    },
    LatestTiltBuild: {
      Version: "",
      Date: "",
      Dev: false,
    },
    FeatureFlags: {
      snapshots: true,
    },
  },
  IsSidebarClosed: false,
  SnapshotLink: "https://snapshots.tilt.dev/foo",
  showSnapshotModal: true,
}

describe("cleanStateForSnapshotPOST", () => {
  it("removes snapshotLink", () => {
    testSnapshot.SnapshotLink = "foo"
    expect(cleanStateForSnapshotPOST(testSnapshot).SnapshotLink).toBe("")
  })
  it("sets showSnapshotModal to false", () => {
    testSnapshot.showSnapshotModal = true
    expect(cleanStateForSnapshotPOST(testSnapshot).showSnapshotModal).toBe(
      false
    )
  })
  it("sets the snapshot feature flag to false", () => {
    testSnapshot.View.FeatureFlags["snapshots"] = true
    let cleanedState = cleanStateForSnapshotPOST(testSnapshot)
    if (cleanedState.View) {
      let features = new Features(cleanedState.View.FeatureFlags)
      expect(features.isEnabled("snapshots")).toBe(false)
    } else {
      throw "Error: View was null"
    }
  })
})
