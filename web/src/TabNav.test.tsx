import { mount } from "enzyme"
import { createMemoryHistory, MemoryHistory } from "history"
import React from "react"
import { act } from "react-dom/test-utils"
import { Router } from "react-router"
import { OverviewNavProvider, TabNavContextConsumer } from "./TabNav"
import { ResourceName } from "./types"

// A fixture to help test the context provider
class Fixture {
  initialTabs: string[]
  nav: any = null
  root: any = null
  history: MemoryHistory = createMemoryHistory()
  validateTab: (res: string) => boolean = () => true

  constructor(initialTabs: string[]) {
    this.initialTabs = initialTabs
  }

  mount() {
    let tabs = this.nav?.tabs || this.initialTabs
    this.root = mount(
      <Router history={this.history}>
        <OverviewNavProvider
          tabsForTesting={tabs}
          validateTab={this.validateTab}
        >
          <TabNavContextConsumer>
            {(capturedNav) => void (this.nav = capturedNav)}
          </TabNavContextConsumer>
        </OverviewNavProvider>
      </Router>
    )
  }

  openResource(name: string, options?: any) {
    act(() => this.nav.openResource(name, options))

    // Enzyme doesn't properly re-render context providers with hooks,
    // so we manually re-render.
    this.mount()
  }
  closeTab(name: string) {
    act(() => this.nav.closeTab(name))

    // Enzyme doesn't properly re-render context providers with hooks,
    // so we manually re-render.
    this.mount()
  }
}

function newFixture(initialTabs: string[]): Fixture {
  let f = new Fixture(initialTabs)
  f.mount()
  return f
}

describe("tabnav", () => {
  afterEach(() => {
    localStorage.clear()
  })

  it("navigates to existing tab on click resource", () => {
    let f = newFixture(["res1", "res2"])
    expect(f.nav.tabs).toEqual(["res1", "res2"])
    expect(f.nav.selectedTab).toEqual("")

    f.openResource("res1")

    expect(f.nav.tabs).toEqual(["res1", "res2"])
    expect(f.nav.selectedTab).toEqual("res1")
    expect(f.history.location.pathname).toEqual("/r/res1/overview")
  })

  it("navigates to new tab on click resource", () => {
    let f = newFixture(["res1", "res2"])
    expect(f.nav.tabs).toEqual(["res1", "res2"])
    expect(f.nav.selectedTab).toEqual("")

    f.openResource("res3")

    expect(f.nav.tabs).toEqual(["res1", "res2", "res3"])
    expect(f.nav.selectedTab).toEqual("res3")
  })

  it("changes selected tab on click existing resource", () => {
    let f = newFixture(["res1", "res2", "res3"])
    expect(f.nav.tabs).toEqual(["res1", "res2", "res3"])
    expect(f.nav.selectedTab).toEqual("")

    f.openResource("res1")

    expect(f.nav.tabs).toEqual(["res1", "res2", "res3"])
    expect(f.nav.selectedTab).toEqual("res1")

    f.openResource("res3")
    expect(f.nav.tabs).toEqual(["res1", "res2", "res3"])
    expect(f.nav.selectedTab).toEqual("res3")
  })

  it("changes selected tab on click new resource", () => {
    let f = newFixture(["res1", "res2", "res3"])
    expect(f.nav.tabs).toEqual(["res1", "res2", "res3"])
    expect(f.nav.selectedTab).toEqual("")

    f.openResource("res2")

    expect(f.nav.tabs).toEqual(["res1", "res2", "res3"])
    expect(f.nav.selectedTab).toEqual("res2")

    f.openResource("res4")
    expect(f.nav.tabs).toEqual(["res1", "res4", "res3"])
    expect(f.nav.selectedTab).toEqual("res4")
  })

  it("open new tab to the right on double-click existing resource", () => {
    let f = newFixture(["res1", "res2", "res3"])
    expect(f.nav.tabs).toEqual(["res1", "res2", "res3"])
    expect(f.nav.selectedTab).toEqual("")

    f.openResource("res1")

    expect(f.nav.tabs).toEqual(["res1", "res2", "res3"])
    expect(f.nav.selectedTab).toEqual("res1")

    f.openResource("res3", { newTab: true })
    expect(f.nav.tabs).toEqual(["res1", "res2", "res3"])
    expect(f.nav.selectedTab).toEqual("res3")
  })

  it("open new tab to the right on double-click new resource", () => {
    let f = newFixture(["res1", "res2", "res3"])
    expect(f.nav.tabs).toEqual(["res1", "res2", "res3"])
    expect(f.nav.selectedTab).toEqual("")

    f.openResource("res1")

    expect(f.nav.tabs).toEqual(["res1", "res2", "res3"])
    expect(f.nav.selectedTab).toEqual("res1")

    f.openResource("res4", { newTab: true })
    expect(f.nav.tabs).toEqual(["res1", "res4", "res2", "res3"])
    expect(f.nav.selectedTab).toEqual("res4")
  })

  it("navigates to the tab on the right when closing", () => {
    let f = newFixture(["res1", "res2", "res3"])
    expect(f.nav.tabs).toEqual(["res1", "res2", "res3"])
    expect(f.nav.selectedTab).toEqual("")

    f.openResource("res2")

    expect(f.nav.tabs).toEqual(["res1", "res2", "res3"])
    expect(f.nav.selectedTab).toEqual("res2")

    f.closeTab("res2")
    expect(f.nav.tabs).toEqual(["res1", "res3"])
    expect(f.nav.selectedTab).toEqual("res3")
    expect(f.history.location.pathname).toEqual("/r/res3/overview")
  })

  it("navigates to home on closing last tab when closing", () => {
    let f = newFixture(["res1"])
    expect(f.nav.tabs).toEqual(["res1"])
    expect(f.nav.selectedTab).toEqual("")

    f.openResource("res1")

    expect(f.nav.tabs).toEqual(["res1"])
    expect(f.nav.selectedTab).toEqual("res1")

    f.closeTab("res1")
    expect(f.nav.tabs).toEqual([ResourceName.all])
    expect(f.nav.selectedTab).toEqual("")
    expect(f.history.location.pathname).toEqual("/overview")
  })

  it("filters tabs that don't validate", () => {
    let f = new Fixture(["res1", "res2"])
    f.validateTab = (res) => res != "res1"
    f.history.location.pathname = "/r/res3/overview"
    f.mount()
    expect(f.nav.tabs).toEqual(["res2", "res3"])
    expect(f.nav.selectedTab).toEqual("res3")
  })

  it("filtering all tabs leaves the all tab", () => {
    let f = new Fixture(["res2", "res3"])
    f.validateTab = (res) => res == "res1"
    f.history.location.pathname = "/"
    f.mount()
    expect(f.nav.tabs).toEqual([ResourceName.all])
    expect(f.nav.selectedTab).toEqual("")
  })
})
