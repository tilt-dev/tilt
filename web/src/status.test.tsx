import { oneResource } from "./testdata.test"
import { zeroTime } from "./time"
import { combinedStatus } from "./status"

function emptyResource() {
  let res = oneResource()
  res.CurrentBuild = { StartTime: zeroTime }
  res.BuildHistory = []
  res.PendingBuildSince = zeroTime
  res.RuntimeStatus = "pending"
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
    res.CurrentBuild = { StartTime: ts }
    res.RuntimeStatus = "ok"
    expect(combinedStatus(res)).toBe("pending")
  })

  it("ok when runtime ok", () => {
    const ts = Date.now().toLocaleString()
    let res = emptyResource()
    res.BuildHistory = [{ StartTime: ts }]
    res.RuntimeStatus = "ok"
    expect(combinedStatus(res)).toBe("ok")
  })

  it("error when runtime error", () => {
    const ts = Date.now().toLocaleString()
    let res = emptyResource()
    res.BuildHistory = [{ StartTime: ts }]
    res.RuntimeStatus = "error"
    expect(combinedStatus(res)).toBe("error")
  })

  it("error when last build error", () => {
    const ts = Date.now().toLocaleString()
    let res = emptyResource()
    res.BuildHistory = [{ StartTime: ts, Error: "error" }]
    res.RuntimeStatus = "ok"
    expect(combinedStatus(res)).toBe("error")
  })
})
