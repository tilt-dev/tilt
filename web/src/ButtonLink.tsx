import React from "react"
import styled from "styled-components"
import { ButtonMixin } from "./ButtonMixin"

let ButtonLinkRoot = styled.a`
  ${ButtonMixin}
`

type ButtonLinkProps = {
  children: any
  href: string
  target?: string
  rel?: string
}

function ButtonLink(props: ButtonLinkProps) {
  return (
    <ButtonLinkRoot
      href={props.href}
      {...(props.target ? { target: props.target } : {})}
      {...(props.rel ? { rel: props.rel } : {})}
    >
      {props.children}
    </ButtonLinkRoot>
  )
}

export default ButtonLink
