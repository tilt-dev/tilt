import AppController from "./AppController"
import PathBuilder from "./PathBuilder"

let fakeSetHistoryLocation = jest.fn()
let fakeSetAppState = jest.fn()

const HUD = {
  setAppState: fakeSetAppState,
  setHistoryLocation: fakeSetHistoryLocation,
}

function flushPromises() {
  return new Promise(resolve => setImmediate(resolve))
}

describe("AppController", () => {
  beforeEach(() => {
    fetchMock.resetMocks()
    fakeSetHistoryLocation.mockReset()
    fakeSetHistoryLocation.mockReset()
  })

  it("sets view from snapshot", async () => {
    fetchMock.mockResponse(JSON.stringify({ view: { resources: [] } }))

    let pb = new PathBuilder("cloud.tilt.dev", "/snapshot/aaaaaaa")
    let ac = new AppController(pb, HUD)
    ac.setStateFromSnapshot()

    await flushPromises()
    expect(fakeSetAppState.mock.calls.length).toBe(1)
    expect(fakeSetHistoryLocation.mock.calls.length).toBe(0)
  })

  it("sets view and path from snapshot", async () => {
    fetchMock.mockResponse(
      JSON.stringify({ view: { resources: [] }, path: "/foo" })
    )

    let pb = new PathBuilder("cloud.tilt.dev", "/snapshot/aaaaaa")
    let ac = new AppController(pb, HUD)
    ac.setStateFromSnapshot()

    await flushPromises()
    expect(fakeSetAppState.mock.calls.length).toBe(2)
    expect(fakeSetHistoryLocation.mock.calls.length).toBe(1)
    expect(fakeSetHistoryLocation.mock.calls[0][0]).toBe("/snapshot/aaaaaa/foo")
  })

  it("sets view and highlight from snapshot", async () => {
    let snapshotHighlight = {
      beginningLogID: "1",
      endingLogID: "6",
    }
    fetchMock.mockResponse(
      JSON.stringify({
        view: { resources: [] },
        snapshotHighlight: snapshotHighlight,
      })
    )

    let pb = new PathBuilder("/**/cloud.tilt.dev", "/snapshot/aaaaaa")
    let ac = new AppController(pb, HUD)
    ac.setStateFromSnapshot()

    await flushPromises()
    expect(fakeSetAppState.mock.calls.length).toBe(4)
    expect(fakeSetAppState.mock.calls[3][0]).toStrictEqual({
      snapshotHighlight: snapshotHighlight,
    })
  })
})
