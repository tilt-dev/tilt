import fetchMock from "fetch-mock"
import AppController from "./AppController"
import PathBuilder from "./PathBuilder"

let fakeSetHistoryLocation = jest.fn()
let fakeOnAppChange = jest.fn()

const HUD = {
  onAppChange: fakeOnAppChange,
  setHistoryLocation: fakeSetHistoryLocation,
}

function flushPromises() {
  return new Promise((resolve) => setTimeout(resolve, 0))
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
    expect(fakeOnAppChange.mock.calls.length).toBe(1)
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
    expect(fakeOnAppChange.mock.calls.length).toBe(1)
    expect(fakeSetHistoryLocation.mock.calls.length).toBe(1)
    expect(fakeSetHistoryLocation.mock.calls[0][0]).toBe("/snapshot/aaaaaa/foo")
  })
})
