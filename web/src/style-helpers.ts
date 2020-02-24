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
  grayLightest = "#93a1a1", // Solarized base1 (darkest content tone)
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
}

let unit = 32

export function SizeUnit(multiplier: number) {
  return `${unit * multiplier}px`
}

// Set sizes expressed in pixels:
export enum Height {
  unit = unit,
  statusHeader = unit * 1.8, // The bar at the top with Pod ID and status
  secondaryNav = unit * 1.2,
  secondaryNavLower = unit * 0.8,
  secondaryNavOverlap = unit * -0.2,
  secondaryNavTwoLevel = unit * 1.8,
  sidebarItem = unit * 1.4, // sync with constants.scss > $sidebar-item
  resourceBar = unit * 1.5,
  statusbar = unit * 1.5,
}
export enum Width {
  sidebar = unit * 10.5, // Sync with constants.scss > $sidebar-width
  secondaryNavItem = unit * 5,
  sidebarTriggerButton = unit * 1.4,
  sidebarCollapsed = unit, // sync with constants.scss > $sidebar-collapsed-width
  badge = unit * 0.6,
  smallScreen = 1500,
}

export const mixinHideOnSmallScreen = `
@media screen and (max-width: ${Width.smallScreen}px) {
  display: none;
}`

export enum ZIndex {
  HUDHeader = 500,
}

export enum AnimDuration {
  default = "0.3s",
}
