import { Hold } from "./Hold"
import LogStore, { LogAlert, LogAlertIndex } from "./LogStore"
import {
  buildStatus,
  combinedStatus,
  PendingBuildDescription,
  runtimeStatus,
} from "./status"
import { oneResource } from "./testdata"
import { zeroTime } from "./time"
import { LogLevel, ResourceStatus, RuntimeStatus, UpdateStatus } from "./types"

class FakeAlertIndex implements LogAlertIndex {
  alerts: { [key: string]: LogAlert[] } = {}

  alertsForSpanId(spanId: string): LogAlert[] {
    return this.alerts[spanId] || []
  }
}

function emptyResource() {
  let res = oneResource({})
  res.status!.currentBuild = { startTime: zeroTime }
  res.status!.buildHistory = []
  res.status!.pendingBuildSince = zeroTime
  res.status!.runtimeStatus = "pending"
  res.status!.updateStatus = "none"
  return res
}

describe("combinedStatus", () => {
  it("pending when no build info", () => {
    let ls = new LogStore()
    let res = emptyResource()
    expect(combinedStatus(buildStatus(res, ls), runtimeStatus(res, ls))).toBe(
      ResourceStatus.Pending
    )
  })

  it("building when current build", () => {
    let ls = new LogStore()
    const ts = Date.now().toLocaleString()
    let res = emptyResource()
    res.status!.updateStatus = UpdateStatus.InProgress
    res.status!.runtimeStatus = RuntimeStatus.Ok
    expect(combinedStatus(buildStatus(res, ls), runtimeStatus(res, ls))).toBe(
      ResourceStatus.Building
    )
  })

  it("healthy when runtime ok", () => {
    let ls = new LogStore()
    let res = emptyResource()
    res.status!.updateStatus = UpdateStatus.Ok
    res.status!.runtimeStatus = RuntimeStatus.Ok
    expect(combinedStatus(buildStatus(res, ls), runtimeStatus(res, ls))).toBe(
      ResourceStatus.Healthy
    )
  })

  it("unhealthy when runtime error", () => {
    let ls = new LogStore()
    let res = emptyResource()
    res.status!.updateStatus = UpdateStatus.Ok
    res.status!.runtimeStatus = RuntimeStatus.Error
    expect(combinedStatus(buildStatus(res, ls), runtimeStatus(res, ls))).toBe(
      ResourceStatus.Unhealthy
    )
  })

  it("unhealthy when last build error", () => {
    let ls = new LogStore()
    let res = emptyResource()
    res.status!.updateStatus = UpdateStatus.Error
    res.status!.runtimeStatus = RuntimeStatus.Ok
    expect(combinedStatus(buildStatus(res, ls), runtimeStatus(res, ls))).toBe(
      ResourceStatus.Unhealthy
    )
  })

  it("building when runtime status error, but also building", () => {
    let ls = new LogStore()
    let res = emptyResource()
    res.status!.updateStatus = UpdateStatus.InProgress
    res.status!.runtimeStatus = RuntimeStatus.Error
    expect(combinedStatus(buildStatus(res, ls), runtimeStatus(res, ls))).toBe(
      ResourceStatus.Building
    )
  })

  it("unhealthy when warning and runtime error", () => {
    let ls = new LogStore()
    let res = emptyResource()
    res.status!.runtimeStatus = RuntimeStatus.Error
    if (!res.status!.k8sResourceInfo) throw new Error("missing k8s info")
    res.status!.k8sResourceInfo.podRestarts = 1
    expect(combinedStatus(buildStatus(res, ls), runtimeStatus(res, ls))).toBe(
      ResourceStatus.Unhealthy
    )
  })

  it("warning when container restarts", () => {
    let ls = new FakeAlertIndex()
    ls.alerts["pod-span-id"] = [{ level: LogLevel.WARN, lineIndex: 1 }]
    const ts = Date.now().toLocaleString()
    let res = emptyResource()
    res.status!.updateStatus = UpdateStatus.Ok
    res.status!.runtimeStatus = RuntimeStatus.Ok
    if (!res.status!.k8sResourceInfo) throw new Error("missing k8s info")
    res.status!.k8sResourceInfo.podRestarts = 1
    res.status!.k8sResourceInfo.spanID = "pod-span-id"
    expect(combinedStatus(buildStatus(res, ls), runtimeStatus(res, ls))).toBe(
      ResourceStatus.Warning
    )
  })

  it("none when n/a runtime status and no builds", () => {
    let ls = new LogStore()
    let res = emptyResource()
    res.status!.updateStatus = UpdateStatus.None
    res.status!.runtimeStatus = RuntimeStatus.NotApplicable
    expect(combinedStatus(buildStatus(res, ls), runtimeStatus(res, ls))).toBe(
      ResourceStatus.None
    )
  })

  it("healthy when n/a runtime status and last build succeeded", () => {
    let ls = new LogStore()
    let res = emptyResource()
    res.status!.runtimeStatus = RuntimeStatus.NotApplicable
    res.status!.updateStatus = UpdateStatus.Ok
    expect(combinedStatus(buildStatus(res, ls), runtimeStatus(res, ls))).toBe(
      ResourceStatus.Healthy
    )
  })

  it("unhealthy when n/a runtime status and last build failed", () => {
    let ls = new LogStore()
    let res = emptyResource()
    res.status!.runtimeStatus = RuntimeStatus.NotApplicable
    res.status!.updateStatus = UpdateStatus.Error
    expect(combinedStatus(buildStatus(res, ls), runtimeStatus(res, ls))).toBe(
      ResourceStatus.Unhealthy
    )
  })
})

describe("PendingBuildDescription", () => {
  it("shows a generic message if there is no hold", () => {
    expect(PendingBuildDescription(null)).toBe("Update: pending")
  })

  it("shows a generic message if there are no dependencies", () => {
    let hold = new Hold({
      reason: "waiting-for-deploy",
      on: [],
    })
    expect(PendingBuildDescription(hold)).toBe("Update: pending")
  })

  it("shows single image name", () => {
    let hold = new Hold({
      reason: "waiting-for-deploy",
      on: [{ group: "", apiVersion: "", kind: "ImageMap", name: "gcr.io/foo" }],
    })
    expect(PendingBuildDescription(hold)).toBe(
      "Update: waiting on image: gcr.io/foo"
    )
  })

  it("shows single resource name", () => {
    let hold = new Hold({
      reason: "waiting-for-deploy",
      on: [{ group: "", apiVersion: "", kind: "UIResource", name: "bar" }],
    })
    expect(PendingBuildDescription(hold)).toBe(
      "Update: waiting on resource: bar"
    )
  })

  it("shows multiple resource names without overflow", () => {
    let hold = new Hold({
      reason: "waiting-for-deploy",
      on: [
        { group: "", apiVersion: "", kind: "UIResource", name: "foo" },
        { group: "", apiVersion: "", kind: "UIResource", name: "bar" },
        { group: "", apiVersion: "", kind: "UIResource", name: "baz" },
      ],
    })
    expect(PendingBuildDescription(hold)).toBe(
      "Update: waiting on resources: foo, bar, baz"
    )
  })

  it("shows multiple image names with overflow", () => {
    let hold = new Hold({
      reason: "waiting-for-deploy",
      on: ["a", "b", "c", "d", "e"].map((x) => ({
        group: "",
        apiVersion: "",
        kind: "ImageMap",
        name: x,
      })),
    })
    expect(PendingBuildDescription(hold)).toBe(
      "Update: waiting on images: a, b, c, and 2 more"
    )
  })

  it("shows cluster name", () => {
    let hold = new Hold({
      reason: "waiting-for-cluster",
      on: [{ group: "", apiVersion: "", kind: "Cluster", name: "default" }],
    })
    expect(PendingBuildDescription(hold)).toBe(
      "Update: waiting on cluster: default"
    )
  })

  it("prefers image over resource", () => {
    let hold = new Hold({
      reason: "waiting-for-deploy",
      on: [
        { group: "", apiVersion: "", kind: "UIResource", name: "foo" },
        { group: "", apiVersion: "", kind: "ImageMap", name: "bar" },
      ],
    })
    expect(PendingBuildDescription(hold)).toBe("Update: waiting on image: bar")
  })

  it("gracefully falls back for unknown types", () => {
    let hold = new Hold({
      reason: "waiting-for-deploy",
      on: [
        { group: "", apiVersion: "", kind: "ThisIsNotARealKind", name: "foo" },
      ],
    })
    expect(PendingBuildDescription(hold)).toBe("Update: waiting on 1 object")
  })
})
