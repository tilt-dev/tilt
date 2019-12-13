// Duplicated styles from constants.scss
export enum Color {
  green = "#20ba31",
  red = "#f6685c",
  yellow = "#fcb41e",
  white = "#ffffff",
  gray = "#93a1a1",
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
