import styled from "styled-components"
import { ApiButton } from "./ApiButton"
import {
  AnimDuration,
  Color,
  Font,
  mixinResetButtonStyle,
} from "./style-helpers"

export const OverviewButtonMixin = `
  ${mixinResetButtonStyle};
  font-family: ${Font.sansSerif};
  display: flex;
  align-items: center;
  padding: 8px 12px;
  margin: 0;

  background: ${Color.grayDark};

  border: 1px solid ${Color.grayLighter};
  box-sizing: border-box;
  border-radius: 4px;
  cursor: pointer;
  transition: color ${AnimDuration.default} ease,
    border-color ${AnimDuration.default} ease;
  color: ${Color.gray7};

  &.isEnabled {
    background: ${Color.gray7};
    color: ${Color.grayDark};
    border-color: ${Color.grayDarker};
  }
  &.isEnabled.isRadio {
    pointer-events: none;
  }

  &:disabled {
    cursor: not-allowed;
    opacity: 0.33;
  }

  & .fillStd {
    fill: ${Color.gray7};
    transition: fill ${AnimDuration.default} ease;
  }
  &.isEnabled .fillStd {
    fill: ${Color.grayDark};
  }

  &:active,
  &:focus {
    outline: none;
    border-color: ${Color.grayLightest};
  }
  &.isEnabled:active,
  &.isEnabled:focus {
    outline: none;
    border-color: ${Color.grayDarkest};
  }

  &:hover {
    color: ${Color.blue};
    border-color: ${Color.blue};
  }
  &:hover .fillStd {
    fill: ${Color.blue};
  }
  &.isEnabled:hover {
    color: ${Color.blueDark};
    border-color: ${Color.blueDark};
  }
  &.isEnabled:hover .fillStd {
    fill: ${Color.blue};
  }
`

export const CustomActionButton = styled(ApiButton)`
  ${OverviewButtonMixin}
`
