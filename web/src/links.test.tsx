import { resolveURL, displayURL } from "./links"

describe("links", () => {
  it("resolves 0.0.0.0", () => {
    expect(resolveURL("http://localhost:8000/x")).toEqual(
      "http://localhost:8000/x"
    )
    expect(resolveURL("http://0.0.0.0:8000/x")).toEqual(
      "http://localhost:8000/x"
    )
  })

  it("handles partial urls", () => {
    expect(resolveURL("localhost:8000")).toEqual("localhost:8000")
  })

  it("strips schemes", () => {
    expect(displayURL("https://localhost:8000")).toEqual("localhost:8000")
    expect(displayURL("http://localhost:8000")).toEqual("localhost:8000")
    expect(displayURL("http://www.google.com")).toEqual("google.com")
  })

  it("strips trailing slash", () => {
    expect(displayURL("http://localhost:8000/")).toEqual("localhost:8000")
    expect(displayURL("http://localhost:8000/foo/")).toEqual(
      "localhost:8000/foo/"
    )
    expect(displayURL("http://localhost/")).toEqual("localhost")
    expect(displayURL("http://localhost/foo/")).toEqual("localhost/foo/")
  })
})
