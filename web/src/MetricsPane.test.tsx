import { mount } from "enzyme"
import fetchMock from "jest-fetch-mock"
import React from "react"
import MetricsPane from "./MetricsPane"
import PathBuilder from "./PathBuilder"

let pb = PathBuilder.forTesting("localhost", "/")

beforeEach(() => {
  fetchMock.resetMocks()
})

it("renders teaser", () => {
  let serving = { mode: "", grafanaHost: "" }
  const root = mount(<MetricsPane pathBuilder={pb} serving={serving} />)

  let element = root.find("input[type='button']")
  expect(element).toHaveLength(1)
  element.simulate("click")

  expect(fetchMock.mock.calls.length).toEqual(1)
  expect(fetchMock.mock.calls[0][0]).toEqual("/api/metrics_opt")
  expect(fetchMock.mock.calls[0][1]?.body).toEqual("local")
})

it("renders loading", () => {
  let serving = { mode: "" }
  mount(<MetricsPane pathBuilder={pb} serving={serving} />)
})

it("renders graphs", () => {
  let serving = { mode: "local", grafanaHost: "localhost:10352" }
  const root = mount(<MetricsPane pathBuilder={pb} serving={serving} />)

  let element = root.find("input[type='button']")
  expect(element).toHaveLength(1)
  element.simulate("click")

  expect(fetchMock.mock.calls.length).toEqual(1)
  expect(fetchMock.mock.calls[0][0]).toEqual("/api/metrics_opt")
  expect(fetchMock.mock.calls[0][1]?.body).toEqual("")
})
