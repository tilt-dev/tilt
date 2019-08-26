import cleanStateForSnapshotPOST from "./snapshot"
import { Snapshot } from "./types"
import { oneResource } from "./testdata.test"

const testSnapshot: Snapshot = {
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
    FeatureFlags: {},
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
})
