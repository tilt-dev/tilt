// Duplicated styles from constants.scss
export enum Color {
  green = "#20ba31",
  blue = "#03c7d3",
  red = "#f6685c",
  yellow = "#fcb41e",
  white = "#ffffff",
  offWhite = "#eef1f1",

  grayLightest = "#93a1a1", // Solarized base1
  grayLight = "#586e75", // Solarized base01
  gray = "#073642", // Solarized base02
  grayDark = "#002b36", // Solarized base03
  grayDarkest = "#001b20",

  text = "#073642",
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

// Heights, expressed in pixels.
let unit = 32
export enum Height {
  unit = unit,
  topBar = unit * 2.25,
  statusbar = unit * 1.5,
  resourceBar = unit * 1.5,
}

export enum Width {
  sidebar = unit * 10,
  sidebarCollapsed = unit * 1.5,
  tabNav = unit * 5, // Match constants.scss > $tabnav-width
}

export function SizeUnit(multiplier: number) {
  let unit = 32
  return `${unit * multiplier}px`
}

export enum ZIndex {
  topFrame = 500,
}

export enum AnimDuration {
  default = "0.3s",
}

// Pod Status
const podStatusError = "Error"
const podStatusCrashLoopBackOff = "CrashLoopBackOff"
const podStatusImagePullBackOff = "ImagePullBackOff"
const podStatusErrImgPull = "ErrImagePull"
const podStatusRunError = "RunContainerError"
const podStatusStartError = "StartError"

function podStatusIsCrash(status: string | undefined) {
  return status === podStatusError || status === podStatusCrashLoopBackOff
}

function podStatusIsError(status: string | undefined) {
  return (
    status === podStatusError ||
    status === podStatusCrashLoopBackOff ||
    status === podStatusImagePullBackOff ||
    status === podStatusErrImgPull ||
    status === podStatusRunError ||
    status === podStatusStartError
  )
}

export { podStatusIsCrash, podStatusIsError }
