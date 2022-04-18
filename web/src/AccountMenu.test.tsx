import { render, screen } from "@testing-library/react"
import React from "react"
import ReactModal from "react-modal"
import { AccountMenuContent } from "./AccountMenu"

beforeEach(() => {
  // Note: `body` is used as the app element _only_ in a test env
  // since the app root element isn't available; in prod, it should
  // be set as the app root so that accessibility features are set correctly
  ReactModal.setAppElement(document.body)
})

it("renders Sign Up button when user is not signed in", () => {
  render(
    <AccountMenuContent
      tiltCloudUsername=""
      tiltCloudSchemeHost="http://cloud.tilt.dev"
      tiltCloudTeamID=""
      tiltCloudTeamName=""
      isSnapshot={false}
    />
  )

  expect(
    screen.getByRole("button", { name: "Link Tilt to Tilt Cloud" })
  ).toBeInTheDocument()
})

it("renders TiltCloud button when user is signed in", () => {
  render(
    <AccountMenuContent
      tiltCloudUsername="amaia"
      tiltCloudSchemeHost="http://cloud.tilt.dev"
      tiltCloudTeamID="cactus inc"
      tiltCloudTeamName=""
      isSnapshot={false}
    />
  )
  expect(
    screen.getByRole("link", { name: "View Tilt Cloud" })
  ).toBeInTheDocument()
})
