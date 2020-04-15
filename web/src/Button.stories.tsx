import React from "react"
import ButtonLink from "./ButtonLink"
import ButtonInput from "./ButtonInput"
import styled from "styled-components"

export default {
  title: "Button",
}

let BG = styled.div`
  width: 100%;
  height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
`

export const buttonLink = () => (
  <BG>
    <ButtonLink href="http://cloud.tilt.dev">View Tilt Cloud</ButtonLink>
  </BG>
)

export const buttonInput = () => (
  <BG>
    <ButtonInput type="submit" value="Sign Up via GitHub" />
  </BG>
)
