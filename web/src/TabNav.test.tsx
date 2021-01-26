import { mount, ReactWrapper } from "enzyme"
import { createMemoryHistory, MemoryHistory } from "history"
import React from "react"
import { act } from "react-dom/test-utils"
import { Router } from "react-router"
import { OverviewNavProvider, TabNav, TabNavContextConsumer } from "./TabNav"
import { ResourceName } from "./types"

type Fixture = { nav: TabNav; root: ReactWrapper; history: MemoryHistory }

function newFixture(tabs: string[]): Fixture {
  let result: any = { nav: null, root: null, history: createMemoryHistory() }
  result.root = mount(
    <Router history={result.history}>
      <OverviewNavProvider tabsForTesting={tabs}>
        <TabNavContextConsumer>
          {(capturedNav) => void (result.nav = capturedNav)}
        </TabNavContextConsumer>
      </OverviewNavProvider>
    </Router>
  )
  return result
}

it("navigates to existing tab on click resource", () => {
  let f = newFixture(["res1", "res2"])
  expect(f.nav.tabs).toEqual(["res1", "res2"])
  expect(f.nav.selectedTab).toEqual("")

  act(() => f.nav.openResource("res1"))

  expect(f.nav.tabs).toEqual(["res1", "res2"])
  expect(f.nav.selectedTab).toEqual("res1")
  expect(f.history.location.pathname).toEqual("/r/res1/overview")
})

it("navigates to new tab on click resource", () => {
  let f = newFixture(["res1", "res2"])
  expect(f.nav.tabs).toEqual(["res1", "res2"])
  expect(f.nav.selectedTab).toEqual("")

  act(() => f.nav.openResource("res3"))

  expect(f.nav.tabs).toEqual(["res1", "res2", "res3"])
  expect(f.nav.selectedTab).toEqual("res3")
})

it("changes selected tab on click existing resource", () => {
  let f = newFixture(["res1", "res2", "res3"])
  expect(f.nav.tabs).toEqual(["res1", "res2", "res3"])
  expect(f.nav.selectedTab).toEqual("")

  act(() => f.nav.openResource("res1"))

  expect(f.nav.tabs).toEqual(["res1", "res2", "res3"])
  expect(f.nav.selectedTab).toEqual("res1")

  act(() => f.nav.openResource("res3"))
  expect(f.nav.tabs).toEqual(["res1", "res2", "res3"])
  expect(f.nav.selectedTab).toEqual("res3")
})

it("changes selected tab on click new resource", () => {
  let f = newFixture(["res1", "res2", "res3"])
  expect(f.nav.tabs).toEqual(["res1", "res2", "res3"])
  expect(f.nav.selectedTab).toEqual("")

  act(() => f.nav.openResource("res2"))

  expect(f.nav.tabs).toEqual(["res1", "res2", "res3"])
  expect(f.nav.selectedTab).toEqual("res2")

  act(() => f.nav.openResource("res4"))
  expect(f.nav.tabs).toEqual(["res1", "res4", "res3"])
  expect(f.nav.selectedTab).toEqual("res4")
})

it("open new tab to the right on double-click existing resource", () => {
  let f = newFixture(["res1", "res2", "res3"])
  expect(f.nav.tabs).toEqual(["res1", "res2", "res3"])
  expect(f.nav.selectedTab).toEqual("")

  act(() => f.nav.openResource("res1"))

  expect(f.nav.tabs).toEqual(["res1", "res2", "res3"])
  expect(f.nav.selectedTab).toEqual("res1")

  act(() => f.nav.openResource("res3", { newTab: true }))
  expect(f.nav.tabs).toEqual(["res1", "res2", "res3"])
  expect(f.nav.selectedTab).toEqual("res3")
})

it("open new tab to the right on double-click new resource", () => {
  let f = newFixture(["res1", "res2", "res3"])
  expect(f.nav.tabs).toEqual(["res1", "res2", "res3"])
  expect(f.nav.selectedTab).toEqual("")

  act(() => f.nav.openResource("res1"))

  expect(f.nav.tabs).toEqual(["res1", "res2", "res3"])
  expect(f.nav.selectedTab).toEqual("res1")

  act(() => f.nav.openResource("res4", { newTab: true }))
  expect(f.nav.tabs).toEqual(["res1", "res4", "res2", "res3"])
  expect(f.nav.selectedTab).toEqual("res4")
})

it("navigates to the tab on the right when closing", () => {
  let f = newFixture(["res1", "res2", "res3"])
  expect(f.nav.tabs).toEqual(["res1", "res2", "res3"])
  expect(f.nav.selectedTab).toEqual("")

  act(() => f.nav.openResource("res2"))

  expect(f.nav.tabs).toEqual(["res1", "res2", "res3"])
  expect(f.nav.selectedTab).toEqual("res2")

  act(() => f.nav.closeTab("res2"))
  expect(f.nav.tabs).toEqual(["res1", "res3"])
  expect(f.nav.selectedTab).toEqual("res3")
  expect(f.history.location.pathname).toEqual("/r/res3/overview")
})

it("navigates to home on closing last tab when closing", () => {
  let f = newFixture(["res1"])
  expect(f.nav.tabs).toEqual(["res1"])
  expect(f.nav.selectedTab).toEqual("")

  act(() => f.nav.openResource("res1"))

  expect(f.nav.tabs).toEqual(["res1"])
  expect(f.nav.selectedTab).toEqual("res1")

  act(() => f.nav.closeTab("res1"))
  expect(f.nav.tabs).toEqual([ResourceName.all])
  expect(f.nav.selectedTab).toEqual("")
  expect(f.history.location.pathname).toEqual("/overview")
})
