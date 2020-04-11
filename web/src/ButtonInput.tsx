import React from "react"
import styled from "styled-components"
import { Button } from "./Button"

let ButtonInputRoot = styled.input`
  ${Button}
  border: 0;
  width: 100%;
`

type ButtonLinkProps = {
  value: string
  type: string
}

function ButtonLink(props: ButtonLinkProps) {
  return <ButtonInputRoot {...props} />
}

export default ButtonLink
