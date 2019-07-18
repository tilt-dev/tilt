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
const podStatusRunError = "RunContainerError"
const podStatusStartError = "StartError"

function podStatusIsCrash(status: string) {
  return status === podStatusError || status === podStatusCrashLoopBackOff
}

  function podStatusErrorFunction(status: string) {
  return (
    status === podStatusError ||
    status === podStatusCrashLoopBackOff ||
    status === podStatusImagePullBackOff ||
    status === podStatusErrImgPull ||
    status === podStatusRunError ||
    status === podStatusStartError
  )
}

export { podStatusIsCrash, podStatusErrorFunction as podStatusErrorFunction}

