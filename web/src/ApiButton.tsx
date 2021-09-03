import {
  ButtonGroup,
  ButtonProps,
  FormControlLabel,
  Icon,
  SvgIcon,
} from "@material-ui/core"
import ArrowDropDownIcon from "@material-ui/icons/ArrowDropDown"
import moment from "moment"
import { useSnackbar } from "notistack"
import React, { useRef, useState } from "react"
import { convertFromNode, convertFromString } from "react-from-dom"
import { Link } from "react-router-dom"
import styled from "styled-components"
import FloatDialog from "./FloatDialog"
import { useHudErrorContext } from "./HudErrorContext"
import {
  InstrumentedButton,
  InstrumentedCheckbox,
  InstrumentedTextField,
} from "./instrumentedComponents"
import { usePathBuilder } from "./PathBuilder"
import { Color, FontSize, SizeUnit } from "./style-helpers"

type UIButton = Proto.v1alpha1UIButton
type UIInputSpec = Proto.v1alpha1UIInputSpec
type UIInputStatus = Proto.v1alpha1UIInputStatus

const ApiButtonFormRoot = styled.div`
  z-index: 20;
`
const ApiButtonFormFooter = styled.div`
  margin-top: ${SizeUnit(0.5)};
  text-align: right;
  font-color: ${Color.grayLighter};
  font-size: ${FontSize.smallester};
`
const ApiIconRoot = styled.div``
export const ApiButtonLabel = styled.div``
export const ApiButtonRoot = styled(ButtonGroup)`
  ${ApiIconRoot} + ${ApiButtonLabel} {
    margin-left: ${SizeUnit(0.25)};
  }
`
export const LogLink = styled(Link)`
  font-size: ${FontSize.smallest};
  padding-left: ${SizeUnit(0.5)};
`

type ApiButtonProps = ButtonProps & {
  className?: string
  uiButton: UIButton
}

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
  value: boolean | undefined
  setValue: (name: string, value: any) => void
}

function ApiButtonInput(props: ApiButtonInputProps) {
  if (props.spec.text) {
    return (
      <InstrumentedTextField
        label={props.spec.label ?? props.spec.name}
        id={props.spec.name}
        defaultValue={props.spec.text?.defaultValue}
        placeholder={props.spec.text?.placeholder}
        value={props.value || props.spec.text?.defaultValue || ""}
        onChange={(e) => props.setValue(props.spec.name!, e.target.value)}
        analyticsName="ui.web.uibutton.inputValue"
        analyticsTags={{ inputType: "text" }}
        fullWidth
      />
    )
  } else if (props.spec.bool) {
    const isChecked = props.value ?? props.spec.bool.defaultValue ?? false
    return (
      <FormControlLabel
        control={
          <InstrumentedCheckbox
            id={props.spec.name}
            checked={isChecked}
            analyticsName="ui.web.uibutton.inputValue"
            analyticsTags={{ inputType: "bool" }}
          />
        }
        label={props.spec.label ?? props.spec.name}
        onChange={(_, checked) => props.setValue(props.spec.name!, checked)}
      />
    )
  } else {
    return (
      <div>{`Error: button input ${props.spec.name} had unsupported type`}</div>
    )
  }
}

type ApiButtonFormProps = {
  uiButton: UIButton
  setInputValue: (name: string, value: any) => void
  getInputValue: (name: string) => any | undefined
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
  setInputValue: (name: string, value: any) => void
  getInputValue: (name: string) => any | undefined
  className?: string
  buttonProps: ButtonProps
}

function ApiButtonWithOptions(props: ApiButtonWithOptionsProps) {
  const [open, setOpen] = useState(false)
  const anchorRef = useRef(null)

  return (
    <>
      <ApiButtonRoot
        ref={anchorRef}
        className={props.className}
        disableRipple={true}
      >
        {props.submit}
        <ApiButtonInputsToggleButton
          size="small"
          onClick={() => {
            setOpen((prevOpen) => !prevOpen)
          }}
          analyticsName="ui.web.uiButton.inputMenu"
          {...props.buttonProps}
        >
          <ArrowDropDownIcon />
        </ApiButtonInputsToggleButton>
      </ApiButtonRoot>
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
    return (
      <ApiIconRoot>
        <SvgIcon component={svg} />
      </ApiIconRoot>
    )
  }

  if (props.iconName) {
    return (
      <ApiIconRoot>
        <Icon>{props.iconName}</Icon>
      </ApiIconRoot>
    )
  }

  return null
}

async function updateButtonStatus(
  button: UIButton,
  inputValues: Map<string, any>
) {
  const toUpdate = {
    metadata: { ...button.metadata },
    status: { ...button.status },
  } as UIButton
  // apiserver's date format time is _extremely_ strict to the point that it requires the full
  // six-decimal place microsecond precision, e.g. .000Z will be rejected, it must be .000000Z
  // so use an explicit RFC3339 moment format to ensure it passes
  toUpdate.status!.lastClickedAt = moment
    .utc()
    .format("YYYY-MM-DDTHH:mm:ss.SSSSSSZ")

  toUpdate.status!.inputs = []
  button.spec!.inputs?.forEach((spec) => {
    const value = inputValues.get(spec.name!)
    if (value !== undefined) {
      let status: UIInputStatus = { name: spec.name }
      if (spec.text) {
        status.text = { value: value }
      } else if (spec.bool) {
        status.bool = { value: value === true }
      }
      toUpdate.status!.inputs!.push(status)
    }
  })

  const url = `/proxy/apis/tilt.dev/v1alpha1/uibuttons/${
    toUpdate.metadata!.name
  }/status`
  const resp = await fetch(url, {
    method: "PUT",
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
    },
    body: JSON.stringify(toUpdate),
  })
  if (resp && resp.status !== 200) {
    const body = await resp.text()
    throw `error submitting button click to api: ${body}`
  }
}

// Renders a UIButton.
// NB: The `Button` in `ApiButton` refers to a UIButton, not an html <button>.
// This can be confusing because each ApiButton consists of one or two <button>s:
// 1. A submit <button>, which fires the button's action.
// 2. Optionally, an options <button>, which allows the user to configure the
//    options used on submit.
export function ApiButton(props: React.PropsWithChildren<ApiButtonProps>) {
  const { className, uiButton, ...buttonProps } = props

  const [loading, setLoading] = useState(false)
  const [inputValues, setInputValues] = useState(new Map<string, any>())

  const { enqueueSnackbar } = useSnackbar()
  const pb = usePathBuilder()

  const { setError } = useHudErrorContext()

  const onClick = async () => {
    // TODO(milas): currently the loading state just disables the button for the duration of
    //  the AJAX request to avoid duplicate clicks - there is no progress tracking at the
    //  moment, so there's no fancy spinner animation or propagation of result of action(s)
    //  that occur as a result of click right now
    setLoading(true)
    try {
      await updateButtonStatus(uiButton, inputValues)
    } catch (err) {
      setError(`Error submitting button click: ${err}`)
      return
    } finally {
      setLoading(false)
    }

    const snackbarLogsLink =
      uiButton.spec?.location?.componentType === "Global" ? (
        <LogLink to="/r/(all)/overview">Global Logs</LogLink>
      ) : (
        <LogLink
          to={pb.encpath`/r/${
            uiButton.spec?.location?.componentID || "(all)"
          }/overview`}
        >
          Resource Logs
        </LogLink>
      )
    enqueueSnackbar(
      <div>
        Triggered button: {uiButton.spec?.text || uiButton.metadata?.name}
        {snackbarLogsLink}
      </div>
    )
  }

  // button text is not included in analytics name since that can be user data
  const button = (
    <InstrumentedButton
      analyticsName={"ui.web.uibutton"}
      onClick={onClick}
      disabled={loading || uiButton.spec?.disabled}
      {...buttonProps}
    >
      {props.children || (
        <>
          <ApiIcon
            iconName={uiButton.spec?.iconName}
            iconSVG={uiButton.spec?.iconSVG}
          />
          <ApiButtonLabel>{uiButton.spec?.text ?? "Button"}</ApiButtonLabel>
        </>
      )}
    </InstrumentedButton>
  )

  if (uiButton.spec?.inputs?.length) {
    const setInputValue = (name: string, value: any) => {
      // We need a `new Map` to ensure the reference changes to force a rerender.
      setInputValues(new Map(inputValues.set(name, value)))
    }
    const getInputValue = (name: string) => inputValues.get(name)

    return (
      <ApiButtonWithOptions
        className={className}
        submit={button}
        uiButton={uiButton}
        setInputValue={setInputValue}
        getInputValue={getInputValue}
        buttonProps={buttonProps}
      />
    )
  } else {
    return (
      <ApiButtonRoot className={className} disableRipple={true}>
        {button}
      </ApiButtonRoot>
    )
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
