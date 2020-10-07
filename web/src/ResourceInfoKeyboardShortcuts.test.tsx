import React from "react"
import { mount } from "enzyme"
import ResourceInfoKeyboardShortcuts from "./ResourceInfoKeyboardShortcuts"
import { fireEvent } from "@testing-library/dom"

function numKeyCode(num: number): number {
  return num + 48
}

type Link = Proto.webviewLink

let openedSnapshotModal = false
let component: any
let endpointUrl = ""
let showSnapshotButton = true
const shortcuts = (endpoints: Link[]) => {
  endpointUrl = ""
  openedSnapshotModal = false
  component = mount(
    <ResourceInfoKeyboardShortcuts
      endpoints={endpoints}
      showSnapshotButton={showSnapshotButton}
      openEndpointUrl={url => (endpointUrl = url)}
      openSnapshotModal={() => (openedSnapshotModal = true)}
    />
  )
}

afterEach(() => {
  showSnapshotButton = true
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
it("handles snapshot shortcut", () => {
  shortcuts([])
  expect(openedSnapshotModal).toEqual(false)
  fireEvent.keyDown(document.body, { key: "s" })
  expect(openedSnapshotModal).toEqual(true)
})
it("does not handle snapshot shortcut when disabled", () => {
  showSnapshotButton = false
  shortcuts([])
  fireEvent.keyDown(document.body, { key: "s" })
  expect(openedSnapshotModal).toEqual(false)
})
