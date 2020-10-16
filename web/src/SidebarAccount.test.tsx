import React from "react"
import SidebarAccount, {
  SidebarAccountRoot,
  SidebarMenuContent,
  MenuContentButtonSignUp,
  MenuContentButtonTiltCloud,
} from "./SidebarAccount"
import ShortcutsDialog from "./ShortcutsDialog"
import { MemoryRouter } from "react-router-dom"
import { mount } from "enzyme"
import { fireEvent } from "@testing-library/dom"
import ReactModal from "react-modal"
import { act } from "react-dom/test-utils"

beforeEach(() => {
  ReactModal.setAppElement(document.body)
})

it("renders nothing on a Tilt Snapshot", () => {
  const root = mount(
    <MemoryRouter initialEntries={["/"]}>
      <SidebarAccount
        tiltCloudUsername=""
        tiltCloudSchemeHost="http://cloud.tilt.dev"
        tiltCloudTeamID=""
        tiltCloudTeamName=""
        isSnapshot={true}
      />
    </MemoryRouter>
  )

  expect(root.find(SidebarAccountRoot)).toHaveLength(0)
  root.unmount()
})

it("renders Sign Up button when user is not signed in", () => {
  const root = mount(
    <SidebarMenuContent
      tiltCloudUsername=""
      tiltCloudSchemeHost="http://cloud.tilt.dev"
      tiltCloudTeamID=""
      tiltCloudTeamName=""
      isSnapshot={false}
    />
  )

  expect(root.find(MenuContentButtonSignUp)).toHaveLength(1)
})

it("renders TiltCloud button when user is signed in", () => {
  const root = mount(
    <SidebarMenuContent
      tiltCloudUsername="amaia"
      tiltCloudSchemeHost="http://cloud.tilt.dev"
      tiltCloudTeamID="cactus inc"
      tiltCloudTeamName=""
      isSnapshot={false}
    />
  )

  expect(root.find(MenuContentButtonTiltCloud)).toHaveLength(1)
})

it("renders shortcuts dialog on ?", () => {
  const root = mount(
    <MemoryRouter initialEntries={["/"]}>
      <SidebarAccount
        tiltCloudUsername="amaia"
        tiltCloudSchemeHost="http://cloud.tilt.dev"
        tiltCloudTeamID="cactus inc"
        tiltCloudTeamName=""
        isSnapshot={false}
      />
    </MemoryRouter>
  )

  expect(root.find(ShortcutsDialog).props().isOpen).toEqual(false)
  act(() => void fireEvent.keyDown(document.body, { key: "?" }))
  expect(root.find(ShortcutsDialog).props().isOpen).toEqual(false)
  root.unmount()
})
