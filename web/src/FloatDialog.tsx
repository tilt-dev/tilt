import React from "react"
import Modal from "react-modal"
import styled from "styled-components"
import { ReactComponent as CloseSvg } from "./assets/svg/close.svg"
import { Color, Font, FontSize } from "./style-helpers"

type props = {
  title: string | React.ReactElement
  isOpen: boolean
  onRequestClose: () => void
  children: any
  style?: any
}

let FloatDialogRoot = styled(Modal)`
  display: flex;
  flex-direction: column;
  background: #ffffff;
  color: ${Color.grayDarkest};
  box-shadow: 3px 3px 4px rgba(0, 0, 0, 0.5);
  border-radius: 8px;
  padding: 16px 20px;
`

let TitleBar = styled.div`
  display: flex;
  flex-direction: row;
  justify-content: space-between;
  align-items: center;
`

let Title = styled.div`
  font-family: ${Font.sansSerif};
  font-weight: 500;
  font-size: 15px;
  line-height: 18px;
`

let HR = styled.hr`
  border-top: 1px dashed ${Color.grayLight};
  margin: 16px -20px;
`

let CloseButton = styled.button`
  display: flex;
  align-items: center;
  border: 0;
  cursor: pointer;
  background-color: white;
  transition: background-color 300ms ease;
  border-radius: 32px 32px;
  padding: 0;

  &:hover,
  &:active {
    background-color: ${Color.grayLightest};
  }
`

let Content = styled.div`
  font: ${Font.monospace};
  font-size: ${FontSize.default};
  line-height: 28px;
`

// A generic dialog that floats in a part of the screen.
// Intended to be attached to a menu button.
export default function FloatDialog(props: props) {
  let title = props.title
  let titleEl = typeof title == "string" ? <Title>{title}</Title> : title
  return (
    <FloatDialogRoot shouldCloseOnEsc={true} {...props}>
      <TitleBar>
        {titleEl}
        <CloseButton onClick={props.onRequestClose}>
          <CloseSvg />
        </CloseButton>
      </TitleBar>
      <HR />
      <Content>{props.children}</Content>
    </FloatDialogRoot>
  )
}
