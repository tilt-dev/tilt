import LogStore, { LogAlert, LogAlertIndex } from "./LogStore"
import { buildStatus, combinedStatus, runtimeStatus } from "./status"
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
  let res = oneResource()
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
    const ts = Date.now().toLocaleString()
    let res = emptyResource()
    res.status!.updateStatus = UpdateStatus.Ok
    res.status!.runtimeStatus = RuntimeStatus.Ok
    expect(combinedStatus(buildStatus(res, ls), runtimeStatus(res, ls))).toBe(
      ResourceStatus.Healthy
    )
  })

  it("unhealthy when runtime error", () => {
    let ls = new LogStore()
    const ts = Date.now().toLocaleString()
    let res = emptyResource()
    res.status!.updateStatus = UpdateStatus.Ok
    res.status!.runtimeStatus = RuntimeStatus.Error
    expect(combinedStatus(buildStatus(res, ls), runtimeStatus(res, ls))).toBe(
      ResourceStatus.Unhealthy
    )
  })

  it("unhealthy when last build error", () => {
    let ls = new LogStore()
    const ts = Date.now().toLocaleString()
    let res = emptyResource()
    res.status!.updateStatus = UpdateStatus.Error
    res.status!.runtimeStatus = RuntimeStatus.Ok
    expect(combinedStatus(buildStatus(res, ls), runtimeStatus(res, ls))).toBe(
      ResourceStatus.Unhealthy
    )
  })

  it("building when runtime status error, but also building", () => {
    let ls = new LogStore()
    const ts = Date.now().toLocaleString()
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
    const ts = Date.now().toLocaleString()
    let res = emptyResource()
    res.status!.runtimeStatus = RuntimeStatus.NotApplicable
    res.status!.updateStatus = UpdateStatus.Ok
    expect(combinedStatus(buildStatus(res, ls), runtimeStatus(res, ls))).toBe(
      ResourceStatus.Healthy
    )
  })

  it("unhealthy when n/a runtime status and last build failed", () => {
    let ls = new LogStore()
    const ts = Date.now().toLocaleString()
    let res = emptyResource()
    res.status!.runtimeStatus = RuntimeStatus.NotApplicable
    res.status!.updateStatus = UpdateStatus.Error
    expect(combinedStatus(buildStatus(res, ls), runtimeStatus(res, ls))).toBe(
      ResourceStatus.Unhealthy
    )
  })
})
