import { withStyles } from "@material-ui/core/styles"
import Switch from "@material-ui/core/Switch"
import React from "react"
import { linkToTiltDocs, TiltDocsPage } from "./constants"
import FloatDialog from "./FloatDialog"
import { Color } from "./style-helpers"

type props = {
  open: boolean
  onClose: () => void
  anchorEl: Element | null
  showUpdate: boolean
  suggestedVersion: string | null | undefined
  isNewInterface: boolean
}

export function showUpdate(view: Proto.webviewView): boolean {
  let session = view?.uiSession?.status
  let runningBuild = session?.runningTiltBuild
  let suggestedVersion = session?.suggestedTiltVersion
  const versionSettings = session?.versionSettings
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

const GreenSwitch = withStyles({
  switchBase: {
    color: Color.gray50,
    "&$checked": {
      color: Color.green,
    },
    "&$checked + $track": {
      backgroundColor: Color.gray30,
    },
  },
  checked: {},
  track: {
    backgroundColor: Color.gray30,
  },
})(Switch)

export default function UpdateDialog(props: props) {
  let { showUpdate, suggestedVersion } = props
  let updateEl: any | null = null
  if (showUpdate) {
    updateEl = (
      <div key="update">
        <span role="img" aria-label="Decorative sparkling stars">
          ✨
        </span>
        &nbsp;
        <a
          href={linkToTiltDocs(TiltDocsPage.Upgrade)}
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
    )
  } else {
    updateEl = (
      <div key="no-update">
        <div>Already on the recommended version!</div>
        <div>
          If you're impatient for more,
          <br />
          subscribe to <a href="https://tilt.dev/subscribe">Tilt News</a>.
        </div>
      </div>
    )
  }

  let title = props.showUpdate ? "Updates Available" : "Update Status"
  return (
    <FloatDialog id="update" title={title} {...props}>
      {updateEl}
    </FloatDialog>
  )
}
