import { fireEvent } from "@testing-library/dom"
import { mount } from "enzyme"
import React from "react"
import { MemoryRouter } from "react-router"
import SidebarItem from "./SidebarItem"
import SidebarKeyboardShortcuts from "./SidebarKeyboardShortcuts"
import { TabNavContextProvider } from "./TabNav"
import { twoResourceView } from "./testdata"
import { ResourceView } from "./types"

var opened: any
let component: any
let triggered: any = false
const shortcuts = (items: SidebarItem[], selected: string) => {
  let tabNav = {
    tabs: [],
    selectedTab: "",
    invalidTab: "",
    openResource: (name: string, options?: { newTab: boolean }) => {
      opened = name
    },
    closeTab: (name: string) => {},
  }
  opened = null

  component = mount(
    <MemoryRouter initialEntries={["/init"]}>
      <TabNavContextProvider value={tabNav}>
        <SidebarKeyboardShortcuts
          items={items}
          selected={selected}
          resourceView={ResourceView.Log}
          onTrigger={() => {
            triggered = true
          }}
        />
      </TabNavContextProvider>
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
  let items = twoResourceView().resources.map((res) => new SidebarItem(res))
  shortcuts(items, "")
  fireEvent.keyDown(document.body, { key: "j" })
  expect(opened).toEqual("vigoda")
})

it("navigates forwards no wrap", () => {
  let items = twoResourceView().resources.map((res) => new SidebarItem(res))
  shortcuts(items, "snack")
  fireEvent.keyDown(document.body, { key: "j" })
  expect(opened).toEqual(null)
})

it("navigates backwards", () => {
  let items = twoResourceView().resources.map((res) => new SidebarItem(res))
  shortcuts(items, "snack")
  fireEvent.keyDown(document.body, { key: "k" })
  expect(opened).toEqual("vigoda")
})

it("navigates backwards no wrap", () => {
  let items = twoResourceView().resources.map((res) => new SidebarItem(res))
  let sks = shortcuts(items, "")
  fireEvent.keyDown(document.body, { key: "k" })
  expect(opened).toEqual(null)
})

it("triggers update", () => {
  let items = twoResourceView().resources.map((res) => new SidebarItem(res))
  let sks = shortcuts(items, "")
  expect(triggered).toEqual(false)
  fireEvent.keyDown(document.body, { key: "r" })
  expect(triggered).toEqual(true)
})
