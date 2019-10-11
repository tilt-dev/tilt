import PathBuilder from "./PathBuilder"

describe("PathBuilder", () => {
  it("handles ports", () => {
    let pb = new PathBuilder("localhost:10350", "/r/fe")
    expect(pb.getDataUrl()).toEqual("ws://localhost:10350/ws/view")
  })

  it("handles room root links", () => {
    let pb = new PathBuilder("localhost", "/view/dead-beef")
    expect(pb.getDataUrl()).toEqual("ws://localhost/join/dead-beef")
    expect(pb.path("/")).toEqual("/view/dead-beef/")
  })

  it("handles snapshots in prod", () => {
    let pb = new PathBuilder("snapshots.tilt.dev", "/snapshot/aaaaaa")
    expect(pb.getDataUrl()).toEqual(
      "https://snapshots.tilt.dev/api/snapshot/aaaaaa"
    )
    expect(pb.path("/foo")).toEqual("/snapshot/aaaaaa/foo")
  })

  it("handles snapshots in dev", () => {
    let pb = new PathBuilder("localhost", "/snapshot/aaaaaa")
    expect(pb.getDataUrl()).toEqual("http://localhost/api/snapshot/aaaaaa")
    expect(pb.path("/foo")).toEqual("/snapshot/aaaaaa/foo")
  })
})
