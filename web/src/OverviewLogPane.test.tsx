import { mount } from "enzyme"
import {
  StyledLines,
  ThreeLines,
  ThreeLinesAllLog,
} from "./OverviewLogPane.stories"

it("renders 3 lines in resource view", () => {
  let root = mount(ThreeLines())
  let el = root.getDOMNode()
  expect(el.querySelectorAll(".LogPaneLine")).toHaveLength(3)
})

it("renders 3 lines in all log view", () => {
  let root = mount(ThreeLinesAllLog())
  let el = root.getDOMNode()
  expect(el.querySelectorAll(".LogPaneLine")).toHaveLength(3)
})

it("escapes html and linkifies", () => {
  let root = mount(StyledLines())
  let el = root.getDOMNode()
  expect(el.querySelectorAll(".LogPaneLine a")).toHaveLength(2)
  expect(el.querySelectorAll(".LogPaneLine button")).toHaveLength(0)
})
