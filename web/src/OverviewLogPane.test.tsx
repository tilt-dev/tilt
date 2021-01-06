import { mount } from "enzyme"
import {
  BuildLogAndRunLog,
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

it("hides build logs", () => {
  let root = mount(BuildLogAndRunLog({}))
  let el = root.getDOMNode()
  expect(el.querySelectorAll(".LogPaneLine")).toHaveLength(20)

  let root2 = mount(BuildLogAndRunLog({ hideBuildLog: true }))
  let el2 = root2.getDOMNode()
  expect(el2.querySelectorAll(".LogPaneLine")).toHaveLength(10)
  expect(el2.innerHTML).toEqual(expect.stringContaining("Vigoda pod line"))
  expect(el2.innerHTML).toEqual(
    expect.not.stringContaining("Vigoda build line")
  )

  let root3 = mount(BuildLogAndRunLog({ hideRunLog: true }))
  let el3 = root3.getDOMNode()
  expect(el3.querySelectorAll(".LogPaneLine")).toHaveLength(10)
  expect(el3.innerHTML).toEqual(expect.not.stringContaining("Vigoda pod line"))
  expect(el3.innerHTML).toEqual(expect.stringContaining("Vigoda build line"))
})
