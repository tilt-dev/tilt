import styled from "styled-components"
import BuildButton from "./BuildButton"
import {
  AnimDuration,
  Color,
  mixinResetButtonStyle,
  overviewItemBorderRadius,
  SizeUnit,
} from "./style-helpers"

export const SidebarBuildButton = styled(BuildButton)`
  ${mixinResetButtonStyle};
  width: ${SizeUnit(1)};
  height: ${SizeUnit(1)};
  background-color: ${Color.grayLighter};
  border-bottom-left-radius: ${overviewItemBorderRadius};
  border-top-right-radius: ${overviewItemBorderRadius};
  display: flex;
  align-items: center;
  flex-shrink: 0;
  justify-content: center;
  opacity: 0;
  pointer-events: none;

  &.is-building {
    display: none;
  }
  &.is-clickable {
    pointer-events: auto;
    cursor: pointer;
  }
  &.is-clickable,
  &.is-queued {
    opacity: 1;
  }
  &.is-selected {
    background-color: ${Color.gray7};
  }
  &:hover {
    background-color: ${Color.grayDark};
  }
  &.is-selected:hover {
    background-color: ${Color.grayLightest};
  }
  & .fillStd {
    transition: fill ${AnimDuration.default} ease;
    fill: ${Color.grayLight};
  }
  &.is-manual .fillStd {
    fill: ${Color.blue};
  }
  &.is-selected .fillStd {
    fill: ${Color.black};
  }
  &:hover .fillStd {
    fill: ${Color.white};
  }
  &.is-selected:hover .fillStd {
    fill: ${Color.blueDark};
  }
  & > svg {
    transition: transform ${AnimDuration.short} linear;
  }
  &:active > svg {
    transform: scale(1.2);
  }
  &.is-queued > svg {
    animation: spin 1s linear infinite;
  }

  // the emphasized svg is bigger, so pad the unemphasized svg to line it up
  padding: 0 0 0 2px;
  &.is-emphasized {
    padding: 0;
  }
`
