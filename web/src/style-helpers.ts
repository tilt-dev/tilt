export enum Color {
  green = "#20ba31",
  blue = "#03c7d3",
  blueLight = "#5edbe3",
  red = "#f6685c",
  yellow = "#fcb41e",
  white = "#ffffff",
  offWhite = "#eef1f1",

  grayLightest = "#93a1a1", // Solarized base1
  grayLight = "#586e75", // Solarized base01
  gray = "#073642", // Solarized base02
  grayDark = "#002b36", // Solarized base03
  grayDarkest = "#001b20",
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
  HUDheader = unit * 3,
  secondaryNav = unit * 1.2,
  logLineSeparator = 3,
  resourceBar = unit * 1.5,
  statusbar = unit * 1.5,
}
export enum Width {
  logLineGutter = 6,
  sidebar = unit * 10,
  sidebarCollapsed = unit * 1.5,
  secondaryNavItem = unit * 5,
  badge = unit * 0.6,
}

export enum ZIndex {
  HUDheader = 500,
}

export enum AnimDuration {
  default = "0.3s",
}
