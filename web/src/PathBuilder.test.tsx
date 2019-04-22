import PathBuilder from "./PathBuilder"

describe("PathBuilder", () => {
  it("handles tilt preview links", () => {
    let pb = new PathBuilder("localhost", "/r/fe/preview")
    expect(pb.getWebsocketUrl()).toEqual("ws://localhost/ws/view")
    expect(pb.path("/")).toEqual("/")
  })

  it("handles room root links", () => {
    let pb = new PathBuilder("localhost", "/view/dead-beef")
    expect(pb.getWebsocketUrl()).toEqual("ws://localhost/join/dead-beef")
    expect(pb.path("/")).toEqual("/view/dead-beef/")
  })

  it("handles room preview links", () => {
    let pb = new PathBuilder("localhost", "/view/deadbeef/r/fe/preview")
    expect(pb.getWebsocketUrl()).toEqual("ws://localhost/join/deadbeef")
    expect(pb.path("/")).toEqual("/view/deadbeef/")
  })
})
