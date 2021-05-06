import fetchMock from "fetch-mock"
import AppController from "./AppController"
import PathBuilder from "./PathBuilder"

let fakeSetHistoryLocation = jest.fn()
let fakeSetAppState = jest.fn()

const HUD = {
  setAppState: fakeSetAppState,
  setHistoryLocation: fakeSetHistoryLocation,
}

function flushPromises() {
  return new Promise((resolve) => setImmediate(resolve))
}

describe("AppController", () => {
  beforeEach(() => {
    fetchMock.reset()
    fakeSetHistoryLocation.mockReset()
    fakeSetHistoryLocation.mockReset()
  })

  it("sets view from snapshot", async () => {
    fetchMock.mock(
      "/api/snapshot/aaaaaaa",
      JSON.stringify({ view: { uiResources: [] } })
    )

    let pb = PathBuilder.forTesting("cloud.tilt.dev", "/snapshot/aaaaaaa")
    let ac = new AppController(pb, HUD)
    ac.setStateFromSnapshot()

    await flushPromises()
    expect(fakeSetAppState.mock.calls.length).toBe(1)
    expect(fakeSetHistoryLocation.mock.calls.length).toBe(0)
  })

  it("sets view and path from snapshot", async () => {
    fetchMock.mock(
      "/api/snapshot/aaaaaa",
      JSON.stringify({ view: { uiResources: [] }, path: "/foo" })
    )

    let pb = PathBuilder.forTesting("cloud.tilt.dev", "/snapshot/aaaaaa")
    let ac = new AppController(pb, HUD)
    ac.setStateFromSnapshot()

    await flushPromises()
    expect(fakeSetAppState.mock.calls.length).toBe(1)
    expect(fakeSetHistoryLocation.mock.calls.length).toBe(1)
    expect(fakeSetHistoryLocation.mock.calls[0][0]).toBe("/snapshot/aaaaaa/foo")
  })

  it("sets view and highlight from snapshot", async () => {
    let snapshotHighlight = {
      beginningLogID: "1",
      endingLogID: "6",
    }
    fetchMock.mock(
      "/api/snapshot/aaaaaa",
      JSON.stringify({
        view: { uiResources: [] },
        snapshotHighlight: snapshotHighlight,
      })
    )

    let pb = PathBuilder.forTesting("/**/cloud.tilt.dev", "/snapshot/aaaaaa")
    let ac = new AppController(pb, HUD)
    ac.setStateFromSnapshot()

    await flushPromises()
    expect(fakeSetAppState.mock.calls.length).toBe(2)
    expect(fakeSetAppState.mock.calls[1][0]).toStrictEqual({
      snapshotHighlight: snapshotHighlight,
    })
  })
})
