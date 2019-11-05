import { oneResource } from "./testdata"
import { zeroTime } from "./time"
import { combinedStatus, warnings } from "./status"

function emptyResource() {
  let res = oneResource()
  res.currentBuild = { startTime: zeroTime }
  res.buildHistory = []
  res.pendingBuildSince = zeroTime
  res.runtimeStatus = "pending"
  return res
}

describe("combinedStatus", () => {
  it("pending when no build info", () => {
    let res = emptyResource()
    expect(combinedStatus(res)).toBe("pending")
  })

  it("pending when current build", () => {
    const ts = Date.now().toLocaleString()
    let res = emptyResource()
    res.currentBuild = { startTime: ts }
    res.runtimeStatus = "ok"
    expect(combinedStatus(res)).toBe("pending")
  })

  it("ok when runtime ok", () => {
    const ts = Date.now().toLocaleString()
    let res = emptyResource()
    res.buildHistory = [{ startTime: ts }]
    res.runtimeStatus = "ok"
    expect(combinedStatus(res)).toBe("ok")
  })

  it("error when runtime error", () => {
    const ts = Date.now().toLocaleString()
    let res = emptyResource()
    res.buildHistory = [{ startTime: ts }]
    res.runtimeStatus = "error"
    expect(combinedStatus(res)).toBe("error")
  })

  it("error when last build error", () => {
    const ts = Date.now().toLocaleString()
    let res = emptyResource()
    res.buildHistory = [{ startTime: ts, error: "error" }]
    res.runtimeStatus = "ok"
    expect(combinedStatus(res)).toBe("error")
  })

  it("container restarts aren't errors", () => {
    const ts = Date.now().toLocaleString()
    let res = emptyResource()
    res.buildHistory = [{ startTime: ts }]
    res.runtimeStatus = "ok"
    if (!res.k8sResourceInfo) throw new Error("missing k8s info")
    res.k8sResourceInfo.podRestarts = 1
    expect(combinedStatus(res)).toBe("ok")
  })

  it("container restarts are warnings", () => {
    const ts = Date.now().toLocaleString()
    let res = emptyResource()
    res.buildHistory = [{ startTime: ts }]
    res.runtimeStatus = "ok"
    if (!res.k8sResourceInfo) throw new Error("missing k8s info")
    res.k8sResourceInfo.podRestarts = 1
    expect(warnings(res)).toEqual(["Container restarted"])
  })
})
