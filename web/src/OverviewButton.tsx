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
    border-color ${AnimDuration.default} ease
    z-index ${AnimDuration.default} ease;
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
    opacity: 0.33;
    border: 1px solid ${ColorRGBA(Color.grayLightest, 0.5)};
    color: ${Color.gray7};
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
