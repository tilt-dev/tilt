import styled from "styled-components"
import BuildButton from "./BuildButton"
import {
  AnimDuration,
  Color,
  mixinResetButtonStyle,
  SizeUnit,
} from "./style-helpers"

export const OverviewTableBuildButton = styled(BuildButton)`
  ${mixinResetButtonStyle};
  display: flex;
  align-items: center;
  justify-content: center;

  & .fillStd {
    transition: fill ${AnimDuration.default} ease;
    fill: ${Color.grayLight};
  }
  &:hover .fillStd {
    fill: ${Color.white};
  }
  & .icon {
    transition: transform ${AnimDuration.short} linear;
    width: ${SizeUnit(0.75)};
    height: ${SizeUnit(0.75)};
  }
  &:active > svg {
    transform: scale(1.2);
  }
  &.is-building > svg {
    animation: spin 1s linear infinite;
  }
  &.is-queued > svg {
    animation: spin 1s linear infinite;
  }
  &.is-manual .fillStd {
    fill: ${Color.blue};
  }

  &.stop-button {
    display: block;
  }
  &.stop-button button {
    min-width: 0;
    border: 0;
    padding: 0;
  }
`
