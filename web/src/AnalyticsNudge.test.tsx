import { shallow } from "enzyme"
import React from "react"
import AnalyticsNudge from "./AnalyticsNudge"

it("shows nudge if needsNudge", () => {
  const component = shallow(<AnalyticsNudge needsNudge={true} />)

  expect(component).toMatchSnapshot()
})

it("hides nudge if !needsNudge and no request made", () => {
  const component = shallow(<AnalyticsNudge needsNudge={false} />)

  expect(component).toMatchSnapshot()
})

it("hides nudge if dismissed, even if needsNudge = true", () => {
  const component = shallow(<AnalyticsNudge needsNudge={true} />)

  component.setState({ dismissed: true })

  expect(component).toMatchSnapshot()
})

it("shows request-in-progress message", () => {
  const component = shallow(<AnalyticsNudge needsNudge={false} />)

  component.setState({ requestMade: true })

  expect(component).toMatchSnapshot()
})

it("shows success message: opt out", () => {
  const component = shallow(<AnalyticsNudge needsNudge={false} />)

  component.setState({ optIn: false, requestMade: true, responseCode: 200 })

  expect(component).toMatchSnapshot()
})

it("shows success message: opt in", () => {
  const component = shallow(<AnalyticsNudge needsNudge={false} />)

  component.setState({ optIn: true, requestMade: true, responseCode: 200 })

  expect(component).toMatchSnapshot()
})

it("shows failure message with response body", () => {
  const component = shallow(<AnalyticsNudge needsNudge={false} />)

  component.setState({
    requestMade: true,
    responseCode: 418,
    responseBody: "something is not right! something is quite wrong!",
  })

  expect(component).toMatchSnapshot()
})

it("hidden if dismissed", () => {
  const component = shallow(<AnalyticsNudge needsNudge={false} />)

  component.setState({ requestMade: true, responseCode: 200, dismissed: true })

  expect(component).toMatchSnapshot()
})
