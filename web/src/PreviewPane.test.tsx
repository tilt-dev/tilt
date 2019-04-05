import React from "react"
import ReactDOM from "react-dom"
import PreviewPane from "./PreviewPane"
import renderer from "react-test-renderer"

it("renders without crashing", () => {
  let div = document.createElement("div")
  Element.prototype.scrollIntoView = jest.fn()
  ReactDOM.render(
    <PreviewPane endpoint="http://localhost:9000" isExpanded={true} />,
    div
  )
  ReactDOM.unmountComponentAtNode(div)
})

it("renders without crashing when there's no endpoint", () => {
  let div = document.createElement("div")
  Element.prototype.scrollIntoView = jest.fn()
  ReactDOM.render(<PreviewPane endpoint="" isExpanded={true} />, div)
  ReactDOM.unmountComponentAtNode(div)
})

it("renders logs", () => {})
