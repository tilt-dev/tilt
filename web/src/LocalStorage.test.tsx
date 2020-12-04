import React from "react"
import { mount } from "enzyme"
import {
  LocalStorageContextProvider,
  localStorageContext,
  makeKey,
  LocalStorageContext,
} from "./LocalStorage"

function lscp(f: (lsc: LocalStorageContext) => any) {
  return mount(
    <LocalStorageContextProvider tiltfileKey={"test"}>
      {
        <localStorageContext.Consumer>
          {(ctx) => f(ctx)}
        </localStorageContext.Consumer>
      }
    </LocalStorageContextProvider>
  )
}

describe("localStorageContext", () => {
  afterEach(() => {
    localStorage.clear()
  })

  it("stores data to local storage", () => {
    lscp((ctx) => {
      ctx.set("test-key", "test-write-value")
      return null
    })

    expect(localStorage.getItem(makeKey("test", "test-key"))).toEqual(
      JSON.stringify("test-write-value")
    )
  })

  it("reads data from local storage", () => {
    localStorage.setItem(
      makeKey("test", "test-key"),
      JSON.stringify("test-read-value")
    )

    let root = lscp((ctx) => {
      return <p>{ctx.get<string>("test-key")}</p>
    })

    expect(root.find("p").text()).toEqual("test-read-value")
  })
})
