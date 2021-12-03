import { mount } from "enzyme"
import React from "react"
import {
  makeKey,
  tiltfileKeyContext,
  usePersistentState,
} from "./BrowserStorage"

describe("localStorageContext", () => {
  afterEach(() => {
    localStorage.clear()
  })

  it("stores data to local storage", () => {
    function ValueSetter() {
      const [value, setValue] = usePersistentState<string>(
        "test-key",
        "initial"
      )
      if (value !== "test-write-value") {
        setValue("test-write-value")
      }
      return null
    }

    mount(
      <tiltfileKeyContext.Provider value={"test"}>
        <ValueSetter />
      </tiltfileKeyContext.Provider>
    )

    expect(localStorage.getItem(makeKey("test", "test-key"))).toEqual(
      JSON.stringify("test-write-value")
    )
  })

  it("reads data from local storage", () => {
    localStorage.setItem(
      makeKey("test", "test-key"),
      JSON.stringify("test-read-value")
    )

    function ValueGetter() {
      const [value, setValue] = usePersistentState<string>(
        "test-key",
        "initial"
      )
      return <p>{value}</p>
    }
    let root = mount(
      <tiltfileKeyContext.Provider value="test">
        <ValueGetter />
      </tiltfileKeyContext.Provider>
    )

    expect(root.find("p").text()).toEqual("test-read-value")
  })
})
