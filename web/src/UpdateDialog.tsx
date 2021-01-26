import FormControlLabel from "@material-ui/core/FormControlLabel"
import FormGroup from "@material-ui/core/FormGroup"
import { withStyles } from "@material-ui/core/styles"
import Switch from "@material-ui/core/Switch"
import React from "react"
import { Link } from "react-router-dom"
import FloatDialog from "./FloatDialog"
import { useInterfaceVersion } from "./InterfaceVersion"
import { usePathBuilder } from "./PathBuilder"
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

const GreenSwitch = withStyles({
  switchBase: {
    color: Color.grayLight,
    "&$checked": {
      color: Color.green,
    },
    "&$checked + $track": {
      backgroundColor: Color.gray,
    },
  },
  checked: {},
  track: {
    backgroundColor: Color.gray,
  },
})(Switch)

export default function UpdateDialog(props: props) {
  let interfaceVersion = useInterfaceVersion()
  let pathBuilder = usePathBuilder()
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

  let isNewDefault = interfaceVersion.isNewDefault()
  let isWrongInterface = isNewDefault != props.isNewInterface
  let title =
    !props.isNewInterface || props.showUpdate
      ? "Updates Available"
      : "Tilt Interface Setting"
  let interfaceSwitch = (
    <GreenSwitch
      checked={!isNewDefault}
      onChange={interfaceVersion.toggleDefault}
    />
  )
  let label = isWrongInterface ? (
    <div>
      Go back to Tilt’s old interface (
      <Link to={pathBuilder.path("/")}>Refresh</Link>)
    </div>
  ) : (
    <div>Go back to Tilt’s old interface</div>
  )
  return (
    <FloatDialog id="update" title={title} {...props}>
      {updateEl}
      <FormGroup>
        <FormControlLabel label={label} control={interfaceSwitch} />
      </FormGroup>
    </FloatDialog>
  )
}
