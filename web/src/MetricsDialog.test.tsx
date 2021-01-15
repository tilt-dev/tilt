import { mount } from "enzyme"
import { Graphs, Loading, Teaser } from "./MetricsDialog.stories"

it("renders metrics teaser", () => {
  mount(Teaser({ onClose: () => {} }))
})
it("renders metrics loading", () => {
  mount(Loading({ onClose: () => {} }))
})
it("renders metrics graphs", () => {
  mount(Graphs({ onClose: () => {} }))
})
