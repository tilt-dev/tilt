import {
  runViewTransition,
  transitionName,
  transitionProps,
} from "./viewTransition"

describe("transitionName", () => {
  it("makes a valid CSS name from any resource name", () => {
    expect(transitionName("row/backend/(Tiltfile)")).toMatch(
      /^vt-row_backend__Tiltfile_-[a-z0-9]+$/
    )
  })

  it("keeps names unique when only punctuation differs", () => {
    expect(transitionName("a.b")).not.toEqual(transitionName("a/b"))
  })
})

describe("runViewTransition", () => {
  afterEach(() => {
    delete (document as any).startViewTransition
  })

  it("makes the change right away in browsers without view transitions", () => {
    const update = jest.fn()
    runViewTransition(document.createElement("div"), update)
    expect(update).toHaveBeenCalled()
  })

  it("names marked elements only for the duration of the transition", async () => {
    const root = document.createElement("div")
    const row = document.createElement("div")
    Object.entries(transitionProps("row")).forEach(([k, v]) =>
      row.setAttribute(k, v)
    )
    row.getBoundingClientRect = () => ({ width: 10, height: 10 } as DOMRect)
    root.append(row)
    // The test DOM doesn't know this CSS property, so watch the calls instead
    let name = ""
    row.style.setProperty = (_prop: string, value: string | null) => {
      name = value || ""
    }
    row.style.removeProperty = () => {
      name = ""
      return ""
    }

    let namedBefore = ""
    let finish = () => {}
    ;(document as any).startViewTransition = (callback: () => void) => {
      namedBefore = name
      callback()
      return { finished: new Promise<void>((resolve) => (finish = resolve)) }
    }

    const update = jest.fn()
    runViewTransition(root, update)

    expect(namedBefore).toBe(transitionName("row"))
    expect(update).toHaveBeenCalled()
    finish()
    await Promise.resolve()
    await Promise.resolve()
    expect(name).toBe("")
  })
})
