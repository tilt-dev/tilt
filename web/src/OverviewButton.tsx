import {
  AnimDuration,
  Color,
  ColorRGBA,
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

  background: transparent;

  border: 1px solid ${ColorRGBA(Color.grayLightest, 0.5)};
  box-sizing: border-box;
  border-radius: 4px;
  cursor: pointer;
  transition: color ${AnimDuration.default} ease,
    border-color ${AnimDuration.default} ease;
  color: ${Color.gray70};

  &.isEnabled {
    background: ${Color.gray70};
    color: ${Color.gray20};
    border-color: ${Color.grayDarker};
  }
  &.isEnabled.isRadio {
    pointer-events: none;
  }

  &:disabled {
    opacity: 0.33;
    border: 1px solid ${ColorRGBA(Color.grayLightest, 0.5)};
    color: ${Color.gray70};
  }

  & .fillStd {
    fill: ${Color.gray70};
    transition: fill ${AnimDuration.default} ease;
  }
  &.isEnabled .fillStd {
    fill: ${Color.gray20};
  }

  &:active,
  &:focus {
    outline: none;
    border-color: ${Color.grayLightest};
  }
  &.isEnabled:active,
  &.isEnabled:focus {
    outline: none;
    border-color: ${Color.gray10};
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
