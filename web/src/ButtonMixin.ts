import { css } from "styled-components"
import { AnimDuration, Color, Font, FontSize } from "./style-helpers"

export const ButtonMixin = css`
  display: inline-block;
  font-family: ${Font.monospace};
  font-size: ${FontSize.default};
  text-decoration: none;
  background-color: ${Color.blue};
  color: ${Color.gray30};
  border-radius: 4px;
  padding: 4px 8px;
  line-height: 21px;
  cursor: pointer;
  transition: background-color ${AnimDuration.default} ease,
    color ${AnimDuration.default} ease;

  &:hover {
    background-color: ${Color.blueLight};
    color: ${Color.gray40};
  }
`
