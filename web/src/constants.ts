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

// Links to Tilt sites
export const TILT_DOCS_LINK = "https://docs.tilt.dev"
export const TILT_PUBLIC_ASSETS_LINK = "https://tilt.dev/assets"

export enum TiltDocsPage {
  DebugFaq = "debug_faq.html",
  Faq = "faq.html",
  Snapshots = "snapshots.html",
  TelemetryFaq = "telemetry_faq.html",
  TiltfileConcepts = "tiltfile_concepts.html",
  TriggerMode = "manual_update_control.html",
  Upgrade = "upgrade.html",
  CustomButtons = "buttons.html",
}

export function linkToTiltDocs(page?: TiltDocsPage, anchor?: string) {
  if (!page) {
    return TILT_DOCS_LINK
  }

  return `${TILT_DOCS_LINK}/${page}${anchor ?? ""}`
}

export function linkToTiltAsset(
  assetType: "svg" | "ico" | "js" | "css" | "img",
  filename: string
) {
  return `${TILT_PUBLIC_ASSETS_LINK}/${assetType}/${filename}`
}

export const DEFAULT_RESOURCE_LIST_LIMIT = 20
export const RESOURCE_LIST_MULTIPLIER = 2
