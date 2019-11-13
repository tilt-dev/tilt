import PathBuilder from "./PathBuilder"

describe("PathBuilder", () => {
  it("handles ports", () => {
    let pb = new PathBuilder("localhost:10350", "/r/fe")
    expect(pb.getDataUrl()).toEqual("ws://localhost:10350/ws/view")
  })

  it("handles snapshots in prod", () => {
    let pb = new PathBuilder("snapshots.tilt.dev", "/snapshot/aaaaaa")
    expect(pb.getDataUrl()).toEqual("/api/snapshot/aaaaaa")
    expect(pb.path("/foo")).toEqual("/snapshot/aaaaaa/foo")
  })

  it("handles snapshots in dev", () => {
    let pb = new PathBuilder("localhost", "/snapshot/aaaaaa")
    expect(pb.getDataUrl()).toEqual("/api/snapshot/aaaaaa")
    expect(pb.path("/foo")).toEqual("/snapshot/aaaaaa/foo")
  })

  it("handles websocket to an alternate host", () => {
    // When tilt starts with --host
    let pb = new PathBuilder("10.205.131.189:10350", "/r/fe")
    expect(pb.getDataUrl()).toEqual("ws://10.205.131.189:10350/ws/view")
  })
})
