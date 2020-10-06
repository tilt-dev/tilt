import { MemoryRouter, useHistory } from "react-router"
import React from "react"
import PathBuilder from "./PathBuilder"
import { mount } from "enzyme"
import { twoResourceView } from "./testdata"
import SidebarItem from "./SidebarItem"
import SidebarKeyboardShortcuts from "./SidebarKeyboardShortcuts"
import { fireEvent } from "@testing-library/dom"

var fakeHistory: any
const pathBuilder = new PathBuilder("localhost", "/")
const shortcuts = (items: SidebarItem[], selected: string) => {
  let CaptureHistory = () => {
    fakeHistory = useHistory()
    return <span />
  }
  mount(
    <MemoryRouter initialEntries={["/init"]}>
      <CaptureHistory />
      <SidebarKeyboardShortcuts
        items={items}
        selected={selected}
        pathBuilder={pathBuilder}
      />
    </MemoryRouter>
  )
}

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
