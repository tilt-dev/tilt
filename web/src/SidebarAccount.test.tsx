import React from "react"
import SidebarAccount, {
  MenuContentButtonSignUp,
  MenuContentButtonTiltCloud,
} from "./SidebarAccount"
import { MemoryRouter } from "react-router-dom"
import { mount } from "enzyme"

it("renders Sign Up button when user is not signed in", () => {
  const root = mount(
    <MemoryRouter initialEntries={["/"]}>
      <SidebarAccount
        tiltCloudUsername=""
        tiltCloudSchemeHost="http://cloud.tilt.dev"
        tiltCloudTeamID=""
      />
    </MemoryRouter>
  )

  expect(root.find(MenuContentButtonSignUp)).toHaveLength(1)
})

it("renders TiltCloud button when user is signed in", () => {
  const root = mount(
    <MemoryRouter initialEntries={["/"]}>
      <SidebarAccount
        tiltCloudUsername="amaia"
        tiltCloudSchemeHost="http://cloud.tilt.dev"
        tiltCloudTeamID="cactus inc"
      />
    </MemoryRouter>
  )

  expect(root.find(MenuContentButtonTiltCloud)).toHaveLength(1)
})
