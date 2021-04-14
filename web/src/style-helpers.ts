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
  white = "#ffffff",

  offWhite = "#eef1f1",
  gray7 = "#CCDADE",
  gray6 = "#7095A0",
  grayLightest = "#93a1a1", // Solarized base1 (darkest content tone)
  grayLighter = "#2D4D55",
  grayLight = "#586e75", // Solarized base01
  gray = "#073642", // Solarized base02
  grayDark = "#002b36", // Solarized base03 (darkest bg tone)
  grayDarker = "#00242d",
  grayDarkest = "#001b20", // Brand
  black = "#000000",

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
let heightUnit = unit // For cases when `Height.unit` shadows `unit`

export function SizeUnit(multiplier: number) {
  return `${unit * multiplier}px`
}

// Set sizes expressed in pixels:
export enum Height {
  unit = heightUnit,
  statusHeader = unit * 1.8, // The bar at the top with Pod ID and status
  secondaryNav = unit * 1.2,
  secondaryNavLower = unit * 0.8,
  secondaryNavOverlap = unit * -0.2,
  secondaryNavTwoLevel = unit * 1.8,
  statusbar = unit * 1.5,
}
export enum Width {
  badge = unit * 0.6,
  secondaryNavItem = unit * 5,
  sidebarTriggerButton = unit,
  sidebar = unit * 10.5, // Sync with constants.scss > $sidebar-width
  sidebarCollapsed = unit,
  statusbar = unit * 1.5, // sync with constants.scss > $statusbar-height
  smallScreen = 1500,

  statusIcon = 22,
  statusIconMarginRight = 10,
}

export const overviewItemBorderRadius = "6px"

export enum ZIndex {
  OverviewItemActions = 2000,
  SidebarMenu = 2000,
  Sidebar = 1000,
  HUDHeader = 500,
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
    background-color: ${ColorRGBA(Color.gray, ColorAlpha.translucent)};
  }
  50% {
    background-color: ${ColorRGBA(Color.gray, ColorAlpha.almostTransparent)};
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
