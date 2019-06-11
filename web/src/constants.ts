// Duplicated styles from constants.scss
export enum Color {
  green = "#20ba31",
  red = "#f6685c",
  yellow = "#fcb41e",
  white = "#ffffff",
  gray = "#93a1a1",
}

// Pod Status
const podStatusError = "Error"
const podStatusCrashLoopBackOff = "CrashLoopBackOff"
const podStatusImagePullBackOff = "ImagePullBackOff"
const podStatusErrImgPull = "ErrImagePull"

export {
  podStatusError,
  podStatusCrashLoopBackOff,
  podStatusImagePullBackOff,
  podStatusErrImgPull,
}
