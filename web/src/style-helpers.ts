import { keyframes } from "styled-components"

export enum Color {
  // Brand Colors
  green = "#20ba31",
  greenLight = "#70d37b",
  blue = "#03c7d3",
  blueLight = "#5edbe3",
  blueDark = "#007d82",
  red = "#f6685c",
  redLight = "#f7aaa4",
  yellow = "#fcb41e",
  yellowLight = "#fdcf6f",
  purple = "#6378ba",
  white = "#ffffff",

  offWhite = "#eef1f1",
  gray70 = "#CCDADE",
  gray60 = "#7095A0",
  gray50 = "#586e75", // Solarized base01
  gray40 = "#2D4D55",
  gray30 = "#073642", // Solarized base02
  gray20 = "#002b36", // Solarized base03 (darkest bg tone)
  gray10 = "#001b20", // Brand
  black = "#000000",

  // Legacy gray scale
  grayLightest = "#93a1a1", // Solarized base1 (darkest content tone)
  grayDarker = "#00242d",

  text = "#073642",
}

export enum ColorAlpha {
  almostTransparent = 0.1,
  translucent = 0.3,
  almostOpaque = 0.7,
}

export function ColorRGBA(hex: string, alpha: number) {
  let r = parseInt(hex.slice(1, 3), 16),
    g = parseInt(hex.slice(3, 5), 16),
    b = parseInt(hex.slice(5, 7), 16)

  return `rgba(${r}, ${g}, ${b}, ${alpha})`
}

export enum Font {
  sansSerif = '"Montserrat", "Open Sans", "Helvetica", "Arial", sans-serif',
  monospace = '"Inconsolata", "Monaco", "Courier New", "Courier", monospace',
}

export enum FontSize {
  largest = "40px",
  large = "26px",
  default = "20px",
  small = "16px",
  smallest = "13px",
  smallester = "10px",
}

let unit = 32

export function SizeUnit(multiplier: number) {
  return `${unit * multiplier}px`
}

export enum Width {
  sidebarDefault = 336,
  smallScreen = 1500,
  statusIcon = 22,
  statusIconMarginRight = 10,
}

export const overviewItemBorderRadius = "6px"

// When adding new z-index values, check to see
// if there are conflicting values in constants.scss
export enum ZIndex {
  ApiButton = 5,
  TableStickyHeader = 999,
  SocketBar = 1000,
}

export enum AnimDuration {
  short = "0.15s",
  default = "0.3s",
  long = "0.6s",
}

export const mixinHideOnSmallScreen = `
@media screen and (max-width: ${Width.smallScreen}px) {
  display: none;
}`

export const mixinResetListStyle = `
  margin: 0;
  list-style: none;
`

export const mixinResetButtonStyle = `
  background-color: transparent;
  border: 0 none;
  padding: 0;
  margin: 0;
  font-family: inherit;
  cursor: pointer;

  // undo Material UI's Button styling
  // TODO - maybe we should be doing it like this? https://material-ui.com/customization/globals/#css
  letter-spacing: normal;
  min-width: 0px;
  text-transform: none;
  line-height: normal;
  font-size: ${FontSize.smallest};
  &:hover {
    background-color: transparent;
  }
`

export const mixinTruncateText = `
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
`

export namespace Glow {
  export const white = keyframes`
    0% {
      background-color: ${ColorRGBA(Color.white, ColorAlpha.translucent)};
    }
    50% {
      background-color: ${ColorRGBA(Color.white, ColorAlpha.almostTransparent)};
    }
  `

  export const dark = keyframes`
    0% {
      background-color: ${ColorRGBA(Color.gray30, ColorAlpha.translucent)};
    }
    50% {
      background-color: ${ColorRGBA(
        Color.gray30,
        ColorAlpha.almostTransparent
      )};
    }
  `

  export const opacity = keyframes`
    0% {
      opacity: 1;
    }
    50% {
      opacity: 0.5;
    }
  `
}

export const spin = keyframes`
  from {
    transform: rotate(0deg);
  }

  to {
    transform: rotate(360deg);
  }
`

export const barberpole = keyframes`
100% {
  background-position: 100% 100%;
}
`
