import { fireEvent } from "@testing-library/dom"
import { mount } from "enzyme"
import React from "react"
import OverviewActionBarKeyboardShortcuts from "./OverviewActionBarKeyboardShortcuts"

function numKeyCode(num: number): number {
  return num + 48
}

type Link = Proto.webviewLink

let component: any
let endpointUrl = ""
const shortcuts = (endpoints: Link[]) => {
  endpointUrl = ""
  component = mount(
    <OverviewActionBarKeyboardShortcuts
      endpoints={endpoints}
      openEndpointUrl={(url) => (endpointUrl = url)}
    />
  )
}

afterEach(() => {
  if (component) {
    component.unmount()
    component = null
  }
})

it("zero endpoint urls", () => {
  shortcuts([])
  fireEvent.keyDown(document.body, { keyCode: numKeyCode(1), shiftKey: true })
  expect(endpointUrl).toEqual("")
})
it("two endpoint urls trigger first", () => {
  shortcuts([
    { url: "https://tilt.dev:4000" },
    { url: "https://tilt.dev:4001" },
  ])
  fireEvent.keyDown(document.body, { keyCode: numKeyCode(1), shiftKey: true })
  expect(endpointUrl).toEqual("https://tilt.dev:4000")
})
it("two endpoint urls trigger second", () => {
  shortcuts([
    { url: "https://tilt.dev:4000" },
    { url: "https://tilt.dev:4001" },
  ])
  fireEvent.keyDown(document.body, { keyCode: numKeyCode(2), shiftKey: true })
  expect(endpointUrl).toEqual("https://tilt.dev:4001")
})
