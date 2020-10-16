import PathBuilder from "./PathBuilder"

describe("PathBuilder", () => {
  it("handles ports", () => {
    let pb = PathBuilder.forTesting("localhost:10350", "/r/fe")
    expect(pb.getDataUrl()).toEqual("ws://localhost:10350/ws/view")
  })

  it("handles snapshots in prod", () => {
    let pb = PathBuilder.forTesting("snapshots.tilt.dev", "/snapshot/aaaaaa")
    expect(pb.getDataUrl()).toEqual("/api/snapshot/aaaaaa")
    expect(pb.path("/foo")).toEqual("/snapshot/aaaaaa/foo")
  })

  it("handles snapshots in dev", () => {
    let pb = PathBuilder.forTesting("localhost", "/snapshot/aaaaaa")
    expect(pb.getDataUrl()).toEqual("/api/snapshot/aaaaaa")
    expect(pb.path("/foo")).toEqual("/snapshot/aaaaaa/foo")
  })

  it("handles websocket to an alternate host", () => {
    // When tilt starts with --host
    let pb = PathBuilder.forTesting("10.205.131.189:10350", "/r/fe")
    expect(pb.getDataUrl()).toEqual("ws://10.205.131.189:10350/ws/view")
  })

  it("handles secure websockets", () => {
    // When run with an https frontend (like ngrok)
    let pb = new PathBuilder({
      protocol: "https:",
      host: "10.205.131.189:10350",
      pathname: "/r/fe",
    })
    expect(pb.getDataUrl()).toEqual("wss://10.205.131.189:10350/ws/view")
  })
})
