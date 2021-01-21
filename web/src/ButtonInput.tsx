import React from "react"
import styled from "styled-components"
import { ButtonMixin } from "./ButtonMixin"

let ButtonInputRoot = styled.input`
  ${ButtonMixin}
  border: 0;
`

type ButtonLinkProps = {
  value: string
  type: string
  onClick?: () => void
}

function ButtonInput(props: ButtonLinkProps) {
  return <ButtonInputRoot {...props} />
}

export default ButtonInput
