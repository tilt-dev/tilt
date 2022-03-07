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
  background-color: ${Color.gray40};
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
    background-color: ${Color.gray70};
  }
  &:hover {
    background-color: ${Color.gray20};
  }
  &.is-selected:hover {
    background-color: ${Color.grayLightest};
  }
  & .fillStd {
    transition: fill ${AnimDuration.default} ease;
    fill: ${Color.gray50};
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
  & .icon {
    transition: transform ${AnimDuration.short} linear;
    width: ${SizeUnit(0.75)};
    height: ${SizeUnit(0.75)};
  }
  &:active > svg {
    transform: scale(1.2);
  }
  &.is-queued > svg {
    animation: spin 1s linear infinite;
  }

  &.stop-button button {
    min-width: 0;
    border: 0;
    padding: 0;
  }
`
