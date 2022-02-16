import { fireEvent } from "@testing-library/dom"
import { mount } from "enzyme"
import React from "react"
import { MemoryRouter } from "react-router"
import LogStore from "./LogStore"
import { ResourceNavContextProvider } from "./ResourceNav"
import SidebarItem from "./SidebarItem"
import SidebarKeyboardShortcuts from "./SidebarKeyboardShortcuts"
import { twoResourceView } from "./testdata"
import { ResourceView } from "./types"

var opened: any
let component: any
let buildStarted: any = false
const shortcuts = (items: SidebarItem[], selected: string) => {
  let resourceNav = {
    selectedResource: "",
    invalidResource: "",
    openResource: (name: string) => {
      opened = name
    },
  }
  opened = null

  component = mount(
    <MemoryRouter initialEntries={["/init"]}>
      <ResourceNavContextProvider value={resourceNav}>
        <SidebarKeyboardShortcuts
          items={items}
          selected={selected}
          resourceView={ResourceView.Log}
          onStartBuild={() => {
            buildStarted = true
          }}
        />
      </ResourceNavContextProvider>
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
  let ls = new LogStore()
  let items = twoResourceView().uiResources.map(
    (res) => new SidebarItem(res, ls)
  )
  shortcuts(items, "")
  fireEvent.keyDown(document.body, { key: "j" })
  expect(opened).toEqual("vigoda")
})

it("navigates forwards no wrap", () => {
  let ls = new LogStore()
  let items = twoResourceView().uiResources.map(
    (res) => new SidebarItem(res, ls)
  )
  shortcuts(items, "snack")
  fireEvent.keyDown(document.body, { key: "j" })
  expect(opened).toEqual(null)
})

it("navigates backwards", () => {
  let ls = new LogStore()
  let items = twoResourceView().uiResources.map(
    (res) => new SidebarItem(res, ls)
  )
  shortcuts(items, "snack")
  fireEvent.keyDown(document.body, { key: "k" })
  expect(opened).toEqual("vigoda")
})

it("navigates backwards no wrap", () => {
  let ls = new LogStore()
  let items = twoResourceView().uiResources.map(
    (res) => new SidebarItem(res, ls)
  )
  let sks = shortcuts(items, "")
  fireEvent.keyDown(document.body, { key: "k" })
  expect(opened).toEqual(null)
})

it("triggers update", () => {
  let ls = new LogStore()
  let items = twoResourceView().uiResources.map(
    (res) => new SidebarItem(res, ls)
  )
  let sks = shortcuts(items, "")
  expect(buildStarted).toEqual(false)
  fireEvent.keyDown(document.body, { key: "r" })
  expect(buildStarted).toEqual(true)
})
