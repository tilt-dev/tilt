import { mount } from "enzyme"
import { OverviewLogComponent } from "./OverviewLogPane"
import {
  BuildLogAndRunLog,
  ManyLines,
  StyledLines,
  ThreeLines,
  ThreeLinesAllLog,
} from "./OverviewLogPane.stories"
import { newFakeRaf, RafProvider, SyncRafProvider } from "./raf"

let logPaneMount = (pane: any) => {
  return mount(<SyncRafProvider>{pane}</SyncRafProvider>)
}

it("renders 3 lines in resource view", () => {
  let root = logPaneMount(<ThreeLines />)
  let el = root.getDOMNode()
  expect(el.querySelectorAll(".LogPaneLine")).toHaveLength(3)
})

it("renders 3 lines in all log view", () => {
  let root = logPaneMount(<ThreeLinesAllLog />)
  let el = root.getDOMNode()
  expect(el.querySelectorAll(".LogPaneLine")).toHaveLength(3)
})

it("escapes html and linkifies", () => {
  let root = logPaneMount(<StyledLines />)
  let el = root.getDOMNode()
  expect(el.querySelectorAll(".LogPaneLine a")).toHaveLength(2)
  expect(el.querySelectorAll(".LogPaneLine button")).toHaveLength(0)
})

it("hides build logs", () => {
  let root = logPaneMount(<BuildLogAndRunLog />)
  let el = root.getDOMNode()
  expect(el.querySelectorAll(".LogPaneLine")).toHaveLength(20)

  let root2 = logPaneMount(<BuildLogAndRunLog hideBuildLog={true} />)
  let el2 = root2.getDOMNode()
  expect(el2.querySelectorAll(".LogPaneLine")).toHaveLength(10)
  expect(el2.innerHTML).toEqual(expect.stringContaining("Vigoda pod line"))
  expect(el2.innerHTML).toEqual(
    expect.not.stringContaining("Vigoda build line")
  )

  let root3 = logPaneMount(<BuildLogAndRunLog hideRunLog={true} />)
  let el3 = root3.getDOMNode()
  expect(el3.querySelectorAll(".LogPaneLine")).toHaveLength(10)
  expect(el3.innerHTML).toEqual(expect.not.stringContaining("Vigoda pod line"))
  expect(el3.innerHTML).toEqual(expect.stringContaining("Vigoda build line"))
})

it("engages autoscrolls on scroll down", () => {
  let fakeRaf = newFakeRaf()
  let root = mount(
    <RafProvider value={fakeRaf}>
      <ManyLines count={100} />
    </RafProvider>
  )
  let component = root
    .find(OverviewLogComponent)
    .instance() as OverviewLogComponent

  component.autoscroll = false
  component.scrollTop = 0
  component.rootRef.current.scrollTop = 1000
  component.onScroll()
  expect(component.scrollTop).toEqual(1000)

  // The scroll has been scheduled, but not engaged yet.
  expect(component.autoscrollRafId).toBeGreaterThan(0)
  expect(component.autoscroll).toEqual(false)

  fakeRaf.invoke(component.autoscrollRafId as number)
  expect(component.autoscroll).toEqual(true)
})
