import { mount } from "enzyme"
import React from "react"
import ReactModal from "react-modal"
import {
  AccountMenuContent,
  MenuContentButtonSignUp,
  MenuContentButtonTiltCloud,
} from "./AccountMenu"

beforeEach(() => {
  ReactModal.setAppElement(document.body)
})

it("renders Sign Up button when user is not signed in", () => {
  const root = mount(
    <AccountMenuContent
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
    <AccountMenuContent
      tiltCloudUsername="amaia"
      tiltCloudSchemeHost="http://cloud.tilt.dev"
      tiltCloudTeamID="cactus inc"
      tiltCloudTeamName=""
      isSnapshot={false}
    />
  )

  expect(root.find(MenuContentButtonTiltCloud)).toHaveLength(1)
})
