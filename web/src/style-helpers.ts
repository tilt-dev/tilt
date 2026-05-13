import { keyframes } from "styled-components"

export enum Color {
  // Brand Colors
  green = "var(--color-green)",
  greenLight = "var(--color-green-light)",
  blue = "var(--color-blue)",
  blueLight = "var(--color-blue-light)",
  blueDark = "var(--color-blue-dark)",
  red = "var(--color-red)",
  redLight = "var(--color-red-light)",
  yellow = "var(--color-yellow)",
  yellowLight = "var(--color-yellow-light)",
  purple = "var(--color-purple)",
  white = "var(--color-white)",

  offWhite = "var(--color-off-white)",
  gray70 = "var(--color-gray-70)",
  gray60 = "var(--color-gray-60)",
  gray50 = "var(--color-gray-50)", // Solarized base01
  gray40 = "var(--color-gray-40)",
  gray30 = "var(--color-gray-30)", // Solarized base02
  gray20 = "var(--color-gray-20)", // Solarized base03 (darkest bg tone)
  gray10 = "var(--color-gray-10)", // Brand
  black = "var(--color-black)",

  // Legacy gray scale
  grayLightest = "var(--color-gray-lightest)", // Solarized base1 (darkest content tone)
  grayDarker = "var(--color-gray-darker)",

  text = "var(--color-gray-30)",
}

// CSS variable names for RGB variants, for use with ColorRGBA
export enum ColorRGB {
  white = "var(--color-white-rgb)",
  black = "var(--color-black-rgb)",
  gray10 = "var(--color-gray-10-rgb)",
  gray20 = "var(--color-gray-20-rgb)",
  gray30 = "var(--color-gray-30-rgb)",
  gray50 = "var(--color-gray-50-rgb)",
  gray60 = "var(--color-gray-60-rgb)",
  grayLightest = "var(--color-gray-lightest-rgb)",
  blue = "var(--color-blue-rgb)",
  green = "var(--color-green-rgb)",
  red = "var(--color-red-rgb)",
  yellow = "var(--color-yellow-rgb)",
}

export enum ColorAlpha {
  almostTransparent = 0.1,
  translucent = 0.3,
  almostOpaque = 0.7,
}

// ColorRGBA now accepts either a hex string or a CSS var RGB reference (e.g., ColorRGB.blue)
export function ColorRGBA(colorOrRgb: string, alpha: number) {
  if (colorOrRgb.startsWith("var(")) {
    // CSS variable RGB reference — use directly
    return `rgba(${colorOrRgb}, ${alpha})`
  }
  // Legacy hex parsing
  let r = parseInt(colorOrRgb.slice(1, 3), 16),
    g = parseInt(colorOrRgb.slice(3, 5), 16),
    b = parseInt(colorOrRgb.slice(5, 7), 16)

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
  sidebarBreakpoint = 320,
  sidebarMinimum = 32,
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
      background-color: ${ColorRGBA(ColorRGB.white, ColorAlpha.translucent)};
    }
    50% {
      background-color: ${ColorRGBA(
        ColorRGB.white,
        ColorAlpha.almostTransparent
      )};
    }
  `

  export const dark = keyframes`
    0% {
      background-color: ${ColorRGBA(ColorRGB.gray30, ColorAlpha.translucent)};
    }
    50% {
      background-color: ${ColorRGBA(
        ColorRGB.gray30,
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
