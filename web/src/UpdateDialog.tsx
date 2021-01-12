import React from "react"
import FloatDialog from "./FloatDialog"

type props = {
  open: boolean
  onClose: () => void
  anchorEl: Element | null
  showUpdate: boolean
  suggestedVersion: string | null | undefined
}

export function showUpdate(view: Proto.webviewView): boolean {
  let runningBuild = view?.runningTiltBuild
  let suggestedVersion = view?.suggestedTiltVersion
  const versionSettings = view?.versionSettings
  const checkUpdates = versionSettings?.checkUpdates
  if (!checkUpdates || !suggestedVersion || !runningBuild) {
    return false
  }

  let { version, dev } = runningBuild
  if (!version || dev) {
    return false
  }

  let versionArray = version.split(".")
  let suggestedArray = suggestedVersion.split(".")
  for (let i = 0; i < versionArray.length; i++) {
    if (suggestedArray[i] > versionArray[i]) {
      return true
    } else if (suggestedArray[i] < versionArray[i]) {
      return false
    }
  }
  return false
}

export default function UpdateDialog(props: props) {
  let { showUpdate, suggestedVersion } = props
  let updateEl = props.showUpdate ? (
    <div>
      <span role="img" aria-label="Decorative sparkling stars">
        ✨
      </span>
      &nbsp;
      <a
        href="https://docs.tilt.dev/upgrade.html"
        target="_blank"
        rel="noopener noreferrer"
      >
        Get Tilt v{suggestedVersion || "?"}!
      </a>
      &nbsp;
      <span role="img" aria-label="Decorative sparkling stars">
        ✨
      </span>
    </div>
  ) : null

  let overview

  return (
    <FloatDialog id="update" title="Updates available" {...props}>
      {updateEl}
    </FloatDialog>
  )
}
