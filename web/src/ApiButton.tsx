import { ButtonGroup, Icon, SvgIcon, TextField } from "@material-ui/core"
import ArrowDropDownIcon from "@material-ui/icons/ArrowDropDown"
import moment from "moment"
import React, { useRef, useState } from "react"
import { convertFromNode, convertFromString } from "react-from-dom"
import styled from "styled-components"
import FloatDialog from "./FloatDialog"
import { InstrumentedButton } from "./instrumentedComponents"
import { Color, FontSize, SizeUnit } from "./style-helpers"

type UIButton = Proto.v1alpha1UIButton
type UIInputSpec = Proto.v1alpha1UIInputSpec
type UIInputStatus = Proto.v1alpha1UIInputStatus

const ApiButtonFormRoot = styled.span`
  z-index: 20;
`
const ApiButtonFormFooter = styled.div`
  margin-top: ${SizeUnit(0.5)};
  text-align: right;
  font-color: ${Color.grayLighter};
  font-size: ${FontSize.smallester};
`
export const ApiButtonLabel = styled.span``

type ApiButtonProps = { className?: string; button: UIButton }

type ApiIconProps = { iconName?: string; iconSVG?: string }

export const ApiButtonInputsToggleButton = styled(InstrumentedButton)`
  &&&& {
    padding: 0 0;
  }
`

const svgElement = (src: string): React.ReactElement => {
  const node = convertFromString(src, {
    selector: "svg",
    type: "image/svg+xml",
    nodeOnly: true,
  }) as SVGSVGElement
  return convertFromNode(node) as React.ReactElement
}

type ApiButtonInputProps = {
  spec: UIInputSpec
  status: UIInputStatus | undefined
  value: string | undefined
  setValue: (name: string, value: string) => void
}

function ApiButtonInput(props: ApiButtonInputProps) {
  return (
    <TextField
      label={props.spec.label ?? props.spec.name}
      id={props.spec.name}
      defaultValue={props.spec.text?.defaultValue}
      placeholder={props.spec.text?.placeholder}
      value={props.value || props.spec.text?.defaultValue || ""}
      onChange={(e) => props.setValue(props.spec.name!, e.target.value)}
      fullWidth
    />
  )
}

type ApiButtonFormProps = {
  uiButton: UIButton
  setInputValue: (name: string, value: string) => void
  getInputValue: (name: string) => string | undefined
}

export function ApiButtonForm(props: ApiButtonFormProps) {
  return (
    <ApiButtonFormRoot>
      {props.uiButton.spec?.inputs?.map((spec) => {
        const name = spec.name!
        const status = props.uiButton.status?.inputs?.find(
          (status) => status.name === name
        )
        const value = props.getInputValue(name)
        return (
          <ApiButtonInput
            key={name}
            spec={spec}
            status={status}
            value={value}
            setValue={props.setInputValue}
          />
        )
      })}
      <ApiButtonFormFooter>(Changes automatically applied)</ApiButtonFormFooter>
    </ApiButtonFormRoot>
  )
}

type ApiButtonWithOptionsProps = {
  submit: JSX.Element
  uiButton: UIButton
  setInputValue: (name: string, value: string) => void
  getInputValue: (name: string) => string | undefined
  className?: string
}

function ApiButtonWithOptions(props: ApiButtonWithOptionsProps) {
  const [open, setOpen] = useState(false)
  const anchorRef = useRef(null)

  return (
    <>
      <ButtonGroup ref={anchorRef} className={props.className}>
        {props.submit}
        <ApiButtonInputsToggleButton
          size="small"
          onClick={() => {
            setOpen((prevOpen) => !prevOpen)
          }}
          analyticsName="ui.web.uiButton.inputs"
        >
          <ArrowDropDownIcon />
        </ApiButtonInputsToggleButton>
      </ButtonGroup>
      <FloatDialog
        open={open}
        onClose={() => {
          setOpen(false)
        }}
        anchorEl={anchorRef.current}
        title={`Options for ${props.uiButton.spec?.text}`}
      >
        <ApiButtonForm {...props} />
      </FloatDialog>
    </>
  )
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

// Renders a UIButton.
// NB: The `Button` in `ApiButton` refers to a UIButton, not an html <button>.
// This can be confusing because each ApiButton consists of one or two <button>s:
// 1. A submit <button>, which fires the button's action.
// 2. Optionally, an options <button>, which allows the user to configure the
//    options used on submit.
export const ApiButton: React.FC<ApiButtonProps> = (props) => {
  const [loading, setLoading] = useState(false)
  const [inputValues, setInputValues] = useState(new Map<string, string>())

  const onClick = async () => {
    const toUpdate = {
      metadata: { ...props.button.metadata },
      status: { ...props.button.status },
    } as UIButton
    // apiserver's date format time is _extremely_ strict to the point that it requires the full
    // six-decimal place microsecond precision, e.g. .000Z will be rejected, it must be .000000Z
    // so use an explicit RFC3339 moment format to ensure it passes
    toUpdate.status!.lastClickedAt = moment
      .utc()
      .format("YYYY-MM-DDTHH:mm:ss.SSSSSSZ")

    toUpdate.status!.inputs = []
    props.button.spec!.inputs?.forEach((spec) => {
      const value = inputValues.get(spec.name!)
      if (value !== undefined) {
        toUpdate.status!.inputs!.push({
          name: spec.name,
          text: { value: value },
        })
      }
    })

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
  const button = (
    <InstrumentedButton
      analyticsName={"ui.web.uibutton"}
      onClick={onClick}
      disabled={loading || props.button.spec?.disabled}
    >
      {props.children || (
        <>
          <ApiIcon
            iconName={props.button.spec?.iconName}
            iconSVG={props.button.spec?.iconSVG}
          />
          <ApiButtonLabel>{props.button.spec?.text ?? "Button"}</ApiButtonLabel>
        </>
      )}
    </InstrumentedButton>
  )

  if (props.button.spec?.inputs?.length) {
    const setInputValue = (name: string, value: string) => {
      // We need a `new Map` to ensure the reference changes to force a rerender.
      setInputValues(new Map(inputValues.set(name, value)))
    }
    const getInputValue = (name: string) => inputValues.get(name)

    return (
      <ApiButtonWithOptions
        className={props.className}
        submit={button}
        uiButton={props.button}
        setInputValue={setInputValue}
        getInputValue={getInputValue}
      />
    )
  } else {
    return <ButtonGroup className={props.className}>{button}</ButtonGroup>
  }
}

export function buttonsForComponent(
  buttons: UIButton[] | undefined,
  componentType: string,
  componentID: string | undefined
): UIButton[] {
  if (!buttons) {
    return []
  }

  return buttons.filter(
    (b) =>
      b.spec?.location?.componentType?.toUpperCase() ===
        componentType.toUpperCase() &&
      b.spec?.location?.componentID === componentID
  )
}
