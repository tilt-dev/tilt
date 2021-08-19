import { Icon, SvgIcon } from "@material-ui/core"
import moment from "moment"
import React, { useState } from "react"
import { convertFromNode, convertFromString } from "react-from-dom"
import { InstrumentedButton } from "./instrumentedComponents"

type UIButton = Proto.v1alpha1UIButton

type ApiButtonProps = { className?: string; button: UIButton }

type ApiIconProps = { iconName?: string; iconSVG?: string }

const svgElement = (src: string): React.ReactElement => {
  const node = convertFromString(src, {
    selector: "svg",
    type: "image/svg+xml",
    nodeOnly: true,
  }) as SVGSVGElement
  return convertFromNode(node) as React.ReactElement
}

export const ApiIcon: React.FC<ApiIconProps> = (props) => {
  if (props.iconSVG) {
    // the material SvgIcon handles accessibility/sizing/colors well but can't accept a raw SVG string
    // create a ReactElement by parsing the source and then use that as the component, passing through
    // the props so that it's correctly styled
    const svgEl = svgElement(props.iconSVG)
    const svg = (props: React.PropsWithChildren<any>) => {
      // merge the props from material-ui while keeping the children of the actual SVG
      return React.cloneElement(svgEl, { ...props }, ...svgEl.props.children)
    }
    return <SvgIcon component={svg} />
  }

  if (props.iconName) {
    return <Icon>{props.iconName}</Icon>
  }

  return null
}

export const ApiButton: React.FC<ApiButtonProps> = (props) => {
  const [loading, setLoading] = useState(false)
  const onClick = async () => {
    const toUpdate = {
      metadata: { ...props.button.metadata },
      status: { ...props.button.status },
    } as UIButton
    // apiserver's date format time is _extremely_ strict to the point that it requires the full
    // six-decimal place microsecond precision, e.g. .000Z will be rejected, it must be .000000Z
    // so use an explicit RFC3339 moment format to ensure it passes
    toUpdate.status!.lastClickedAt = moment().format(
      "YYYY-MM-DDTHH:mm:ss.SSSSSSZ"
    )

    // TODO(milas): currently the loading state just disables the button for the duration of
    //  the AJAX request to avoid duplicate clicks - there is no progress tracking at the
    //  moment, so there's no fancy spinner animation or propagation of result of action(s)
    //  that occur as a result of click right now
    setLoading(true)
    const url = `/proxy/apis/tilt.dev/v1alpha1/uibuttons/${
      toUpdate.metadata!.name
    }/status`
    try {
      await fetch(url, {
        method: "PUT",
        headers: {
          Accept: "application/json",
          "Content-Type": "application/json",
        },
        body: JSON.stringify(toUpdate),
      })
    } finally {
      setLoading(false)
    }
  }
  // button text is not included in analytics name since that can be user data
  return (
    <InstrumentedButton
      analyticsName={"ui.web.uibutton"}
      onClick={onClick}
      disabled={loading || props.button.spec?.disabled}
      className={props.className}
    >
      {props.children || (
        <>
          <ApiIcon
            iconName={props.button.spec?.iconName}
            iconSVG={props.button.spec?.iconSVG}
          />
          {props.button.spec?.text ?? "Button"}
        </>
      )}
    </InstrumentedButton>
  )
}

export function buttonsForComponent(
  buttons: UIButton[] | undefined,
  componentType: string,
  componentID: string | undefined
): UIButton[] {
  if (!buttons) {
    return []
  }

  return buttons
    .filter(
      (b) =>
        b.spec?.location?.componentType?.toUpperCase() ===
          componentType.toUpperCase() &&
        b.spec?.location?.componentID === componentID
    )
    .sort((a, b) =>
      (a.metadata?.creationTimestamp || "").localeCompare(
        b.metadata?.creationTimestamp || ""
      )
    )
}
