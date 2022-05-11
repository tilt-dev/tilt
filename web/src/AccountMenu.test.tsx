import { render, screen } from "@testing-library/react"
import React from "react"
import { AccountMenuContent, AccountMenuDialog } from "./AccountMenu"
import Features, { FeaturesTestProvider, Flag } from "./feature"

describe("AccountMenuDialog", () => {
  it("does render when offline snapshot creation is NOT enabled", () => {
    const features = new Features({ [Flag.OfflineSnapshotCreation]: false })
    render(
      <AccountMenuDialog
        open={true}
        onClose={() => {}}
        anchorEl={document.body}
        tiltCloudUsername=""
        tiltCloudSchemeHost="http://cloud.tilt.dev"
        tiltCloudTeamID=""
        tiltCloudTeamName=""
      />,
      {
        wrapper: ({ children }) => (
          <FeaturesTestProvider value={features} children={children} />
        ),
      }
    )

    expect(
      screen.getByRole("button", { name: "Link Tilt to Tilt Cloud" })
    ).toBeInTheDocument()
  })

  it("does NOT render when offline snapshot creation is enabled", () => {
    const features = new Features({ [Flag.OfflineSnapshotCreation]: true })
    render(
      <AccountMenuDialog
        open={true}
        onClose={() => {}}
        anchorEl={document.body}
        tiltCloudUsername=""
        tiltCloudSchemeHost="http://cloud.tilt.dev"
        tiltCloudTeamID=""
        tiltCloudTeamName=""
      />,
      {
        wrapper: ({ children }) => (
          <FeaturesTestProvider value={features} children={children} />
        ),
      }
    )

    expect(
      screen.queryByRole("button", { name: "Link Tilt to Tilt Cloud" })
    ).toBeNull()
  })

  describe("AccountMenuContent", () => {
    it("renders Sign Up button when user is not signed in", () => {
      render(
        <AccountMenuContent
          tiltCloudUsername=""
          tiltCloudSchemeHost="http://cloud.tilt.dev"
          tiltCloudTeamID=""
          tiltCloudTeamName=""
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
        />
      )
      expect(
        screen.getByRole("link", { name: "View Tilt Cloud" })
      ).toBeInTheDocument()
    })
  })
})
