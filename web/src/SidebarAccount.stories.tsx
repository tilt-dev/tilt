import React from "react"
import SidebarAccount from "./SidebarAccount"

export default {
  title: "SidebarAccount",
}

export const Default = () => (
  <div style={{ width: "600px" }}>
    <SidebarAccount
      isSnapshot={false}
      tiltCloudUsername={""}
      tiltCloudSchemeHost={""}
      tiltCloudTeamID={""}
      tiltCloudTeamName={""}
    />
  </div>
)

export const LoggedIn = () => (
  <div style={{ width: "600px" }}>
    <SidebarAccount
      isSnapshot={false}
      tiltCloudUsername={"pusheen"}
      tiltCloudSchemeHost={"https://cloud.tilt.dev/"}
      tiltCloudTeamID={"deadcat"}
      tiltCloudTeamName={"pugsheen"}
    />
  </div>
)
