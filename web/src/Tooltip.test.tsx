import { Button } from "@material-ui/core"
import { mount } from "enzyme"
import React from "react"
import { act } from "react-test-renderer"
import { tiltfileKeyContext } from "./LocalStorage"
import TiltTooltip, { TiltInfoTooltip } from "./Tooltip"

function mountWithContext(children: JSX.Element) {
  return mount(
    <tiltfileKeyContext.Provider value="test">
      {children}
    </tiltfileKeyContext.Provider>
  )
}

describe("TiltInfoTooltip", () => {
  beforeEach(() => {
    localStorage.clear()
  })

  afterEach(() => {
    localStorage.clear()
  })

  it("hides info button when clicked", () => {
    const root = mountWithContext(
      <TiltInfoTooltip title="Hello!" dismissId="test-tooltip" open={true} />
    )

    expect(root.find(TiltTooltip).length).toEqual(1)

    act(() => {
      root.find(Button).simulate("click")
    })
    root.update()

    // the tooltip is gone!
    expect(root.find(TiltTooltip).length).toEqual(0)

    // and the setting is in localStorage
    expect(localStorage.getItem("tooltip-dismissed-test-tooltip")).toEqual(
      "true"
    )
  })
})
