import { combinedStatus, warnings } from "./status"
import { oneResource } from "./testdata"
import { zeroTime } from "./time"
import { ResourceStatus, RuntimeStatus, UpdateStatus } from "./types"

function emptyResource() {
  let res = oneResource()
  res.currentBuild = { startTime: zeroTime }
  res.buildHistory = []
  res.pendingBuildSince = zeroTime
  res.runtimeStatus = "pending"
  res.updateStatus = "none"
  return res
}

describe("combinedStatus", () => {
  it("pending when no build info", () => {
    let res = emptyResource()
    expect(combinedStatus(res)).toBe(ResourceStatus.Pending)
  })

  it("building when current build", () => {
    const ts = Date.now().toLocaleString()
    let res = emptyResource()
    res.updateStatus = UpdateStatus.InProgress
    res.runtimeStatus = RuntimeStatus.Ok
    expect(combinedStatus(res)).toBe(ResourceStatus.Building)
  })

  it("healthy when runtime ok", () => {
    const ts = Date.now().toLocaleString()
    let res = emptyResource()
    res.updateStatus = UpdateStatus.Ok
    res.runtimeStatus = RuntimeStatus.Ok
    expect(combinedStatus(res)).toBe(ResourceStatus.Healthy)
  })

  it("unhealthy when runtime error", () => {
    const ts = Date.now().toLocaleString()
    let res = emptyResource()
    res.updateStatus = UpdateStatus.Ok
    res.runtimeStatus = RuntimeStatus.Error
    expect(combinedStatus(res)).toBe(ResourceStatus.Unhealthy)
  })

  it("unhealthy when last build error", () => {
    const ts = Date.now().toLocaleString()
    let res = emptyResource()
    res.updateStatus = UpdateStatus.Error
    res.runtimeStatus = RuntimeStatus.Ok
    expect(combinedStatus(res)).toBe(ResourceStatus.Unhealthy)
  })

  it("building when runtime status error, but also building", () => {
    const ts = Date.now().toLocaleString()
    let res = emptyResource()
    res.updateStatus = UpdateStatus.InProgress
    res.runtimeStatus = RuntimeStatus.Error
    expect(combinedStatus(res)).toBe(ResourceStatus.Building)
  })

  it("unhealthy when warning and runtime error", () => {
    let res = emptyResource()
    res.runtimeStatus = RuntimeStatus.Error
    if (!res.k8sResourceInfo) throw new Error("missing k8s info")
    res.k8sResourceInfo.podRestarts = 1
    expect(combinedStatus(res)).toBe(ResourceStatus.Unhealthy)
  })

  it("warning when container restarts", () => {
    const ts = Date.now().toLocaleString()
    let res = emptyResource()
    res.updateStatus = UpdateStatus.Ok
    res.runtimeStatus = RuntimeStatus.Ok
    if (!res.k8sResourceInfo) throw new Error("missing k8s info")
    res.k8sResourceInfo.podRestarts = 1
    expect(combinedStatus(res)).toBe(ResourceStatus.Warning)
    expect(warnings(res)).toEqual(["Container restarted"])
  })

  it("none when n/a runtime status and no builds", () => {
    let res = emptyResource()
    res.updateStatus = UpdateStatus.None
    res.runtimeStatus = RuntimeStatus.NotApplicable
    expect(combinedStatus(res)).toBe(ResourceStatus.None)
  })

  it("healthy when n/a runtime status and last build succeeded", () => {
    const ts = Date.now().toLocaleString()
    let res = emptyResource()
    res.runtimeStatus = RuntimeStatus.NotApplicable
    res.updateStatus = UpdateStatus.Ok
    expect(combinedStatus(res)).toBe(ResourceStatus.Healthy)
  })

  it("unhealthy when n/a runtime status and last build failed", () => {
    const ts = Date.now().toLocaleString()
    let res = emptyResource()
    res.runtimeStatus = RuntimeStatus.NotApplicable
    res.updateStatus = UpdateStatus.Error
    expect(combinedStatus(res)).toBe(ResourceStatus.Unhealthy)
  })
})
