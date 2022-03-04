import React from "react"
import styled from "styled-components"
import { AccountMenuContent } from "./AccountMenu"
import { Color } from "./style-helpers"

let Container = styled.div`
  display: flex;
  flex-direction: column;
  background: #ffffff;
  color: ${Color.gray10};
  box-shadow: 3px 3px 4px rgba(0, 0, 0, 0.5);
  border-radius: 8px;
  padding: 16px 20px;
  width: 400px;
`

export default {
  title: "New UI/Shared/AccountMenu",
  decorators: [
    (Story: any) => (
      <Container>
        <Story />
      </Container>
    ),
  ],
}

export const SignedOut = () => (
  <AccountMenuContent
    tiltCloudUsername=""
    tiltCloudSchemeHost="http://cloud.tilt.dev"
    tiltCloudTeamID=""
    tiltCloudTeamName=""
    isSnapshot={false}
  />
)

export const SignedIn = () => (
  <AccountMenuContent
    tiltCloudUsername="amaia"
    tiltCloudSchemeHost="http://cloud.tilt.dev"
    tiltCloudTeamID="cactus inc"
    tiltCloudTeamName=""
    isSnapshot={false}
  />
)
