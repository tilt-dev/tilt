import { css } from "styled-components"
import { Font, FontSize, Color, SizeUnit, AnimDuration } from "./style-helpers"

export const ButtonMixin = css`
  display: flex;
  font-family: ${Font.sansSerif};
  font-size: ${FontSize.small};
  text-decoration: none;
  background-color: ${Color.blue};
  color: ${Color.white};
  align-items: center;
  justify-content: center;
  border-radius: ${SizeUnit(0.15)};
  padding-left: ${SizeUnit(0.75)};
  padding-right: ${SizeUnit(0.75)};
  padding-top: ${SizeUnit(0.5)};
  padding-bottom: ${SizeUnit(0.5)};
  line-height: 1;
  cursor: pointer;
  max-width: ${SizeUnit(10)}; // Beyond which it looks less and less button-like
  transition: background-color ${AnimDuration.default} ease;

  &:hover {
    background-color: ${Color.blueLight};
  }
`
