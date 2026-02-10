import { render, RenderResult } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import React from "react"
import { MemoryRouter } from "react-router"
import LogStore from "./LogStore"
import { ResourceNavContextProvider } from "./ResourceNav"
import SidebarItem from "./SidebarItem"
import SidebarKeyboardShortcuts from "./SidebarKeyboardShortcuts"
import { nResourceView } from "./testdata"
import { ResourceView } from "./types"

describe("SidebarKeyboardShortcuts", () => {
  const logStore = new LogStore()
  const items = nResourceView(2).uiResources.map(
    (r) => new SidebarItem(r, logStore)
  )
  let rerender: RenderResult["rerender"]
  let openResourceSpy: jest.Mock
  let onStartBuildSpy: jest.Mock

  beforeEach(() => {
    openResourceSpy = jest.fn()
    onStartBuildSpy = jest.fn()

    const resourceNavValue = {
      selectedResource: "",
      invalidResource: "",
      openResource: openResourceSpy,
    }

    rerender = render(
      <SidebarKeyboardShortcuts
        items={items}
        selected=""
        resourceView={ResourceView.Log}
        onStartBuild={onStartBuildSpy}
      />,
      {
        wrapper: ({ children }) => (
          <MemoryRouter
            initialEntries={["/init"]}
            future={{ v7_startTransition: true, v7_relativeSplatPath: true }}
          >
            <ResourceNavContextProvider value={resourceNavValue}>
              {children}
            </ResourceNavContextProvider>
          </MemoryRouter>
        ),
      }
    ).rerender
  })

  it("navigates forwards on 'j'", () => {
    expect(openResourceSpy).not.toHaveBeenCalled()

    userEvent.keyboard("j")

    expect(openResourceSpy).toHaveBeenCalledWith(items[0].name)
  })

  it("navigates forwards on 'j' without wrapping", () => {
    // Select the last resource item
    rerender(
      <SidebarKeyboardShortcuts
        items={items}
        selected={items[1].name}
        resourceView={ResourceView.Log}
        onStartBuild={onStartBuildSpy}
      />
    )

    userEvent.keyboard("j")

    expect(openResourceSpy).not.toHaveBeenCalled()
  })

  it("navigates backward on 'k'", () => {
    // Select the last resource item
    rerender(
      <SidebarKeyboardShortcuts
        items={items}
        selected={items[1].name}
        resourceView={ResourceView.Log}
        onStartBuild={onStartBuildSpy}
      />
    )

    userEvent.keyboard("k")

    expect(openResourceSpy).toHaveBeenCalledWith(items[0].name)
  })

  it("navigates backward on 'k' without wrapping", () => {
    userEvent.keyboard("k")

    expect(openResourceSpy).not.toHaveBeenCalled()
  })

  it("triggers update on 'r'", () => {
    expect(onStartBuildSpy).not.toHaveBeenCalled()

    userEvent.keyboard("r")

    expect(onStartBuildSpy).toHaveBeenCalled()
  })
})
