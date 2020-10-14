import React from "react"
import { storiesOf } from "@storybook/react"
import SidebarAccount from "./SidebarAccount"

storiesOf("SidebarAccount", module)
  .add("default", () => (
    <div style={{ width: "350px" }}>
      <SidebarAccount
        isSnapshot={false}
        tiltCloudUsername={""}
        tiltCloudSchemeHost={""}
        tiltCloudTeamID={""}
        tiltCloudTeamName={""}
      />
    </div>
  ))
  .add("logged-in", () => (
    <div style={{ width: "350px" }}>
      <SidebarAccount
        isSnapshot={false}
        tiltCloudUsername={"pusheen"}
        tiltCloudSchemeHost={"https://cloud.tilt.dev/"}
        tiltCloudTeamID={"deadcat"}
        tiltCloudTeamName={"pugsheen"}
      />
    </div>
  ))
