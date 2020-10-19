import { MemoryRouter, useHistory } from "react-router"
import React from "react"
import PathBuilder from "./PathBuilder"
import { mount } from "enzyme"
import { twoResourceView } from "./testdata"
import SidebarItem from "./SidebarItem"
import SidebarKeyboardShortcuts from "./SidebarKeyboardShortcuts"
import { fireEvent } from "@testing-library/dom"

var fakeHistory: any
const pathBuilder = PathBuilder.forTesting("localhost", "/")
let component: any
let triggered: any = false
const shortcuts = (items: SidebarItem[], selected: string) => {
  let CaptureHistory = () => {
    fakeHistory = useHistory()
    return <span />
  }
  triggered = false

  component = mount(
    <MemoryRouter initialEntries={["/init"]}>
      <CaptureHistory />
      <SidebarKeyboardShortcuts
        items={items}
        selected={selected}
        pathBuilder={pathBuilder}
        onTrigger={() => {
          triggered = true
        }}
      />
    </MemoryRouter>
  )
}

afterEach(() => {
  if (component) {
    component.unmount()
    component = null
  }
})

it("navigates forwards", () => {
  let items = twoResourceView().resources.map(res => new SidebarItem(res))
  shortcuts(items, "")
  fireEvent.keyDown(document.body, { key: "j" })
  expect(fakeHistory.location.pathname).toEqual("/r/vigoda")
})

it("navigates forwards no wrap", () => {
  let items = twoResourceView().resources.map(res => new SidebarItem(res))
  shortcuts(items, "snack")
  fireEvent.keyDown(document.body, { key: "j" })
  expect(fakeHistory.location.pathname).toEqual("/init")
})

it("navigates backwards", () => {
  let items = twoResourceView().resources.map(res => new SidebarItem(res))
  shortcuts(items, "snack")
  fireEvent.keyDown(document.body, { key: "k" })
  expect(fakeHistory.location.pathname).toEqual("/r/vigoda")
})

it("navigates backwards no wrap", () => {
  let items = twoResourceView().resources.map(res => new SidebarItem(res))
  let sks = shortcuts(items, "")
  fireEvent.keyDown(document.body, { key: "k" })
  expect(fakeHistory.location.pathname).toEqual("/init")
})

it("triggers update", () => {
  let items = twoResourceView().resources.map(res => new SidebarItem(res))
  let sks = shortcuts(items, "")
  expect(triggered).toEqual(false)
  fireEvent.keyDown(document.body, { key: "r" })
  expect(triggered).toEqual(true)
})
