import React from "react"
import AnalyticsNudge from "./AnalyticsNudge"
import { shallow } from "enzyme"

it("hides nudge if !needsNudge and no state", () => {
  const component = shallow(<AnalyticsNudge needsNudge={false} />)

  expect(component).toMatchSnapshot()
})

it("shows nudge if needsNudge", () => {
  const component = shallow(<AnalyticsNudge needsNudge={true} />)

  expect(component).toMatchSnapshot()
})

it("shows request-in-progress message", () => {
  const component = shallow(<AnalyticsNudge needsNudge={false} />)

  component.setState({ requestMade: true })

  expect(component).toMatchSnapshot()
})

it("shows success message", () => {
  const component = shallow(<AnalyticsNudge needsNudge={false} />)

  component.setState({ requestMade: true, responseCode: 200 })

  expect(component).toMatchSnapshot()
})

it("shows failure message", () => {
  const component = shallow(<AnalyticsNudge needsNudge={false} />)

  component.setState({ requestMade: true, responseCode: 418 })

  expect(component).toMatchSnapshot()
})

it("hidden if dismissed", () => {
  const component = shallow(<AnalyticsNudge needsNudge={false} />)

  component.setState({ requestMade: true, responseCode: 200, dismissed: true })

  expect(component).toMatchSnapshot()
})
