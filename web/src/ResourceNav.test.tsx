import { render } from "@testing-library/react"
import { mount, ReactWrapper } from "enzyme"
import { createMemoryHistory, MemoryHistory } from "history"
import React from "react"
import { act } from "react-dom/test-utils"
import { Router } from "react-router"
import {
  ResourceNav,
  ResourceNavContextConsumer,
  ResourceNavProvider,
  useResourceNav,
} from "./ResourceNav"
import { ResourceName } from "./types"

// A fixture to help test the context provider
class Fixture {
  nav?: ResourceNav
  root?: ReactWrapper
  history: MemoryHistory = createMemoryHistory()
  validateResource: (res: string) => boolean = () => true

  mount() {
    this.root = mount(
      <Router history={this.history}>
        <ResourceNavProvider validateResource={this.validateResource}>
          <ResourceNavContextConsumer>
            {(capturedNav) => void (this.nav = capturedNav)}
          </ResourceNavContextConsumer>
        </ResourceNavProvider>
      </Router>
    )
  }

  openResource(name: string) {
    act(() => this.nav?.openResource(name))
  }
}

function newFixture(): Fixture {
  let f = new Fixture()
  f.mount()
  return f
}

describe("resourceNav", () => {
  it("navigates to resource on click", () => {
    let f = newFixture()
    expect(f.nav?.selectedResource).toEqual("")

    f.openResource("res1")

    expect(f.nav?.selectedResource).toEqual("res1")
    expect(f.history.location.pathname).toEqual("/r/res1/overview")
  })

  it("filters resources that don't validate", () => {
    let f = new Fixture()
    f.validateResource = (res) => res == "res1"
    f.history.location.pathname = "/r/res3/overview"
    f.mount()
    expect(f.nav?.selectedResource).toEqual("")
    expect(f.nav?.invalidResource).toEqual("res3")
  })

  it("always validates the 'all' resource", () => {
    let f = new Fixture()
    f.validateResource = (res) => false
    f.history.location.pathname = `/r/${ResourceName.all}/overview`
    f.mount()
    expect(f.nav?.selectedResource).toEqual(ResourceName.all)
    expect(f.nav?.invalidResource).toEqual("")
  })

  it("encodes resource names", () => {
    let f = newFixture()
    f.openResource("foo/bar")
    expect(f.nav?.selectedResource).toEqual("foo/bar")
    expect(f.history.location.pathname).toEqual("/r/foo%2Fbar/overview")
  })

  // Make sure that useResourceNav() doesn't break memoization.
  it("memoizes renders", () => {
    let renderCount = 0
    let FakeEl = React.memo(() => {
      useResourceNav()
      renderCount++
      return <div></div>
    })

    let history = createMemoryHistory()
    let validateResource = () => true
    let { rerender } = render(
      <Router history={history}>
        <ResourceNavProvider validateResource={validateResource}>
          <FakeEl />
        </ResourceNavProvider>
      </Router>
    )

    expect(renderCount).toEqual(1)

    // Make sure we don't re-render on a no-op history update.
    rerender(
      <Router history={history}>
        <ResourceNavProvider validateResource={validateResource}>
          <FakeEl />
        </ResourceNavProvider>
      </Router>
    )
    expect(renderCount).toEqual(1)

    // Make sure we do re-render on a real location update.
    act(() => history.push("/r/foo"))
    expect(renderCount).toEqual(2)
  })
})
