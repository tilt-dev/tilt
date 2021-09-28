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
import { usePersistentState } from "./LocalStorage"
import { usePathBuilder } from "./PathBuilder"
import { Color, FontSize, SizeUnit } from "./style-helpers"
import { apiTimeFormat, tiltApiPut } from "./tiltApi"

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
// MUI makes it tricky to get cursor: not-allowed on disabled buttons
// https://material-ui.com/components/buttons/#cursor-not-allowed
export const ApiButtonRoot = styled(ButtonGroup)<{ disabled?: boolean }>`
  ${(props) =>
    props.disabled &&
    `
    cursor: not-allowed;
  `}
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
  value: any | undefined
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
  } else if (props.spec.hidden) {
    return null
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
}

function ApiButtonWithOptions(props: ApiButtonWithOptionsProps & ButtonProps) {
  const [open, setOpen] = useState(false)
  const anchorRef = useRef(null)

  const {
    submit,
    uiButton,
    setInputValue,
    getInputValue,
    ...buttonProps
  } = props

  return (
    <>
      <ApiButtonRoot
        ref={anchorRef}
        className={props.className}
        disableRipple={true}
        disabled={buttonProps.disabled}
      >
        {props.submit}
        <ApiButtonInputsToggleButton
          size="small"
          onClick={() => {
            setOpen((prevOpen) => !prevOpen)
          }}
          analyticsName="ui.web.uiButton.inputMenu"
          aria-label={`Open ${props.uiButton.spec?.text} options`}
          {...buttonProps}
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

// returns metadata + button status w/ the specified input buttons
function buttonStatusWithInputs(
  button: UIButton,
  inputValues: { [name: string]: any }
): UIButton {
  const result = {
    metadata: { ...button.metadata },
    status: { ...button.status },
  } as UIButton

  result.status!.lastClickedAt = apiTimeFormat(moment.utc())

  result.status!.inputs = []
  button.spec!.inputs?.forEach((spec) => {
    const value = inputValues[spec.name!]
    if (value !== undefined) {
      let status: UIInputStatus = { name: spec.name }
      if (spec.text) {
        status.text = { value: value }
      } else if (spec.bool) {
        status.bool = { value: value === true }
      } else if (spec.hidden) {
        status.hidden = { value: spec.hidden.value }
      }
      result.status!.inputs!.push(status)
    }
  })

  return result
}

async function updateButtonStatus(
  button: UIButton,
  inputValues: { [name: string]: any }
) {
  const toUpdate = buttonStatusWithInputs(button, inputValues)

  await tiltApiPut("uibuttons", "status", toUpdate)
}

function setHiddenInputs(
  uiButton: UIButton,
  inputValues: { [name: string]: any }
) {
  uiButton.spec?.inputs?.forEach((i) => {
    if (i.hidden && i.name) {
      inputValues[i.name] = i.hidden.value
    }
  })
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
  const [inputValues, setInputValues] = usePersistentState<{
    [name: string]: any
  }>(`apibutton-${uiButton.metadata?.name}`, {})

  const { enqueueSnackbar } = useSnackbar()
  const pb = usePathBuilder()

  const { setError } = useHudErrorContext()

  const onClick = async () => {
    // TODO(milas): currently the loading state just disables the button for the duration of
    //  the AJAX request to avoid duplicate clicks - there is no progress tracking at the
    //  moment, so there's no fancy spinner animation or propagation of result of action(s)
    //  that occur as a result of click right now
    setLoading(true)
    setHiddenInputs(uiButton, inputValues)
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

  const disabled = loading || uiButton.spec?.disabled

  // button text is not included in analytics name since that can be user data
  const button = (
    <InstrumentedButton
      analyticsName={"ui.web.uibutton"}
      onClick={onClick}
      disabled={disabled}
      aria-label={`Trigger ${uiButton.spec?.text}`}
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
      // Copy to a new object so that the reference changes to force a rerender.
      setInputValues({ ...inputValues, [name]: value })
    }
    const getInputValue = (name: string) => inputValues[name]

    return (
      <ApiButtonWithOptions
        className={className}
        submit={button}
        uiButton={uiButton}
        setInputValue={setInputValue}
        getInputValue={getInputValue}
        aria-label={uiButton.spec?.text}
        // use-case-wise, it'd probably be better to leave the options button enabled
        // regardless of the submit button's state.
        // However, that's currently a low-impact difference, and this is a really
        // cheap way to ensure the styling matches.
        disabled={disabled}
        {...buttonProps}
      />
    )
  } else {
    return (
      <ApiButtonRoot
        className={className}
        disableRipple={true}
        aria-label={uiButton.spec?.text}
        disabled={disabled}
      >
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
