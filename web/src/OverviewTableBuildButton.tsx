import styled from "styled-components"
import BuildButton from "./BuildButton"
import { AnimDuration, Color, mixinResetButtonStyle } from "./style-helpers"

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
  & > svg {
    transition: transform ${AnimDuration.short} linear;
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

  // the emphasized svg is bigger, so pad the unemphasized svg to line it up
  padding: 0 0 0 5px;
  &.is-emphasized {
    padding: 0;
  }
`
