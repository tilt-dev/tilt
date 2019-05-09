import PathBuilder from "./PathBuilder"

describe("PathBuilder", () => {
  it("handles tilt preview links", () => {
    let pb = new PathBuilder("localhost", "/r/fe/preview")
    expect(pb.getWebsocketUrl()).toEqual("ws://localhost/ws/view")
    expect(pb.path("/")).toEqual("/")
  })

  it("handles ports", () => {
    let pb = new PathBuilder("localhost:10350", "/r/fe/preview")
    expect(pb.getWebsocketUrl()).toEqual("ws://localhost:10350/ws/view")
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

  it("handles https", () => {
    let pb = new PathBuilder("sail.tilt.dev", "/r/fe/preview")
    expect(pb.getWebsocketUrl()).toEqual("wss://sail.tilt.dev/ws/view")
    expect(pb.path("/")).toEqual("/")
  })
})
