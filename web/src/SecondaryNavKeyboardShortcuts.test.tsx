import { MemoryRouter, useHistory } from "react-router"
import React from "react"
import { createMemoryHistory } from "history"
import { mount } from "enzyme"
import SecondaryNavKeyboardShortcuts from "./SecondaryNavKeyboardShortcuts"
import { fireEvent } from "@testing-library/dom"

var fakeHistory: any
const shortcuts = (logUrl: string, alertsUrl: string, facetsUrl: string) => {
  let CaptureHistory = () => {
    fakeHistory = useHistory()
    return <span />
  }

  mount(
    <MemoryRouter initialEntries={["/init"]}>
      <CaptureHistory />
      <SecondaryNavKeyboardShortcuts
        logUrl={logUrl}
        alertsUrl={alertsUrl}
        facetsUrl={facetsUrl}
      />
    </MemoryRouter>
  )
}

it("navigates to logs", () => {
  shortcuts("/logs", "/alerts", "/facets")
  fireEvent.keyDown(document.body, { key: "1" })
  expect(fakeHistory.location.pathname).toEqual("/logs")
})

it("does not navigate to logs with ctrlKey", () => {
  shortcuts("/logs", "/alerts", "/facets")
  fireEvent.keyDown(document.body, { key: "1", ctrlKey: true })
  expect(fakeHistory.location.pathname).toEqual("/init")
})

it("navigates to alerts", () => {
  shortcuts("/logs", "/alerts", "/facets")
  fireEvent.keyDown(document.body, { key: "2" })
  expect(fakeHistory.location.pathname).toEqual("/alerts")
})

it("navigates to facets", () => {
  shortcuts("/logs", "/alerts", "/facets")
  fireEvent.keyDown(document.body, { key: "3" })
  expect(fakeHistory.location.pathname).toEqual("/facets")
})

it("does not navigate to facets", () => {
  shortcuts("/logs", "/alerts", "")
  fireEvent.keyDown(document.body, { key: "3" })
  expect(fakeHistory.location.pathname).toEqual("/init")
})
