import {
  ButtonClassKey,
  ButtonGroup,
  ButtonProps,
  FormControlLabel,
  Icon,
  InputLabel,
  MenuItem,
  Select,
  SvgIcon,
} from "@material-ui/core"
import ArrowDropDownIcon from "@material-ui/icons/ArrowDropDown"
import { ClassNameMap } from "@material-ui/styles"
import moment from "moment"
import { useSnackbar } from "notistack"
import React, {
  PropsWithChildren,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react"
import { convertFromNode, convertFromString } from "react-from-dom"
import { Link } from "react-router-dom"
import styled from "styled-components"
import { annotations } from "./annotations"
import { ReactComponent as CloseSvg } from "./assets/svg/close.svg"
import { usePersistentState } from "./BrowserStorage"
import FloatDialog from "./FloatDialog"
import { useHudErrorContext } from "./HudErrorContext"
import {
  InstrumentedButton,
  InstrumentedCheckbox,
  InstrumentedTextField,
} from "./instrumentedComponents"
import { usePathBuilder } from "./PathBuilder"
import {
  AnimDuration,
  Color,
  Font,
  FontSize,
  SizeUnit,
  ZIndex,
} from "./style-helpers"
import { apiTimeFormat, tiltApiPut } from "./tiltApi"
import { UIButton, UIInputSpec, UIInputStatus } from "./types"

/**
 * Note on nomenclature: both `ApiButton` and `UIButton` are used to refer to
 * custom action buttons here. On the Tilt backend, these are generally called
 * `UIButton`s, but to avoid confusion on the frontend, (where there are many
 * UI buttons,) they're generally called `ApiButton`s.
 */

// Types
type ApiButtonProps = ButtonProps & {
  className?: string
  uiButton: UIButton
}

type ApiIconProps = { iconName?: string; iconSVG?: string }

type ApiButtonInputProps = {
  spec: UIInputSpec
  status: UIInputStatus | undefined
  value: any | undefined
  setValue: (name: string, value: any) => void
}

type ApiButtonElementProps = ButtonProps & {
  text: string
  confirming: boolean
  disabled: boolean
  iconName?: string
  iconSVG?: string
}

// UIButtons for a location, sorted into types
export type ButtonSet = {
  default: UIButton[]
  toggleDisable?: UIButton
  stopBuild?: UIButton
}

function newButtonSet(): ButtonSet {
  return { default: [] }
}

export enum ApiButtonType {
  Global = "Global",
  Resource = "Resource",
}

export enum ApiButtonToggleState {
  On = "on",
  Off = "off",
}

// Constants
export const UIBUTTON_SPEC_HASH = "uibuttonspec-hash"
export const UIBUTTON_ANNOTATION_TYPE = "tilt.dev/uibutton-type"
export const UIBUTTON_GLOBAL_COMPONENT_ID = "nav"
export const UIBUTTON_TOGGLE_DISABLE_TYPE = "DisableToggle"
export const UIBUTTON_TOGGLE_INPUT_NAME = "action"
export const UIBUTTON_STOP_BUILD_TYPE = "StopBuild"

// Styles
const ApiButtonFormRoot = styled.div`
  z-index: ${ZIndex.ApiButton};
`
const ApiButtonFormFooter = styled.div`
  text-align: right;
  color: ${Color.gray40};
  font-size: ${FontSize.smallester};
`
const ApiIconRoot = styled.span``
export const ApiButtonLabel = styled.span``
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

export const confirmingButtonStateMixin = `
&.confirming {
  background-color: ${Color.red};
  border-color: ${Color.gray30};
  color: ${Color.black};

  &:hover,
  &:active,
  &:focus {
    background-color: ${Color.red};
    border-color: ${Color.redLight};
    color: ${Color.black};
  }

  .fillStd {
    fill: ${Color.black} !important; /* TODO (lizz): find this style source! */
  }
}
`

/* Manually manage the border that both left and right
 * buttons share on the edge between them, so border
 * color changes work as expected
 */
export const confirmingButtonGroupBorderMixin = `
&.leftButtonInGroup {
  border-right: 0;

  &:active + .rightButtonInGroup,
  &:focus + .rightButtonInGroup,
  &:hover + .rightButtonInGroup {
    border-left-color: ${Color.redLight};
  }
}
`

const ApiButtonElementRoot = styled(InstrumentedButton)`
  ${confirmingButtonStateMixin}
  ${confirmingButtonGroupBorderMixin}
`

const inputLabelMixin = `
font-family: ${Font.monospace};
font-size: ${FontSize.small};
color: ${Color.gray10};
`

const ApiButtonInputLabel = styled(InputLabel)`
  ${inputLabelMixin}
  margin-top: ${SizeUnit(1 / 2)};
  margin-bottom: ${SizeUnit(1 / 4)};
`

const ApiButtonInputTextField = styled(InstrumentedTextField)`
  margin-bottom: ${SizeUnit(1 / 2)};

  .MuiOutlinedInput-root {
    background-color: ${Color.offWhite};
  }

  .MuiOutlinedInput-input {
    ${inputLabelMixin}
    border: 1px solid ${Color.gray70};
    border-radius: ${SizeUnit(0.125)};
    transition: border-color ${AnimDuration.default} ease;
    padding: ${SizeUnit(0.2)} ${SizeUnit(0.4)};

    &:hover {
      border-color: ${Color.gray40};
    }

    &:focus,
    &:active {
      border: 1px solid ${Color.gray20};
    }
  }
`

const ApiButtonInputFormControlLabel = styled(FormControlLabel)`
  ${inputLabelMixin}
  margin-left: unset;
`

const ApiButtonInputCheckbox = styled(InstrumentedCheckbox)`
  &.MuiCheckbox-root,
  &.Mui-checked {
    color: ${Color.gray40};
  }
`

export const ApiButtonInputsToggleButton = styled(InstrumentedButton)`
  &&&& {
    margin-left: unset; /* Override any margins passed down through "className" props */
    padding: 0 0;
  }
`

function buttonType(b: UIButton): string {
  return annotations(b)[UIBUTTON_ANNOTATION_TYPE]
}

const svgElement = (src: string): React.ReactElement => {
  const node = convertFromString(src, {
    selector: "svg",
    type: "image/svg+xml",
    nodeOnly: true,
  }) as SVGSVGElement
  return convertFromNode(node) as React.ReactElement
}

function ApiButtonInput(props: ApiButtonInputProps) {
  if (props.spec.text) {
    return (
      <>
        <ApiButtonInputLabel htmlFor={props.spec.name}>
          {props.spec.label ?? props.spec.name}
        </ApiButtonInputLabel>
        <ApiButtonInputTextField
          id={props.spec.name}
          placeholder={props.spec.text?.placeholder}
          value={props.value ?? props.spec.text?.defaultValue ?? ""}
          onChange={(e) => props.setValue(props.spec.name!, e.target.value)}
          variant="outlined"
          fullWidth
        />
      </>
    )
  } else if (props.spec.bool) {
    const isChecked = props.value ?? props.spec.bool.defaultValue ?? false
    return (
      <ApiButtonInputFormControlLabel
        control={
          <ApiButtonInputCheckbox id={props.spec.name} checked={isChecked} />
        }
        label={props.spec.label ?? props.spec.name}
        onChange={(_, checked) => props.setValue(props.spec.name!, checked)}
      />
    )
  } else if (props.spec.hidden) {
    return null
  } else if (props.spec.choice) {
    // @ts-ignore
    const currentChoice = props.value ?? props.spec.choice.choices?.at(0)
    const menuItems = []
    // @ts-ignore
    for (let choice of props.spec.choice?.choices) {
      menuItems.push(
        <MenuItem key={choice} value={choice}>
          {choice}
        </MenuItem>
      )
    }
    return (
      <>
        <ApiButtonInputFormControlLabel
          control={
            <Select
              id={props.spec.name}
              value={currentChoice}
              label={props.spec.label ?? props.spec.name}
            >
              {menuItems}
            </Select>
          }
          label={props.spec.label ?? props.spec.name}
          onChange={(e) => {
            // @ts-ignore
            props.setValue(props.spec.name!, e.target.value as string)
          }}
          aria-label={props.spec.label ?? props.spec.name}
        />
      </>
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
  text: string
}

function ApiButtonWithOptions(props: ApiButtonWithOptionsProps & ButtonProps) {
  const [open, setOpen] = useState(false)
  const anchorRef = useRef(null)

  const {
    submit,
    uiButton,
    setInputValue,
    getInputValue,
    text,
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
          {...buttonProps}
          size="small"
          onClick={() => {
            setOpen((prevOpen) => !prevOpen)
          }}
          aria-label={`Open ${text} options`}
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
        title={`Options for ${text}`}
      >
        <ApiButtonForm {...props} />
      </FloatDialog>
    </>
  )
}

export const ApiIcon = ({ iconName, iconSVG }: ApiIconProps) => {
  if (iconSVG) {
    // the material SvgIcon handles accessibility/sizing/colors well but can't accept a raw SVG string
    // create a ReactElement by parsing the source and then use that as the component, passing through
    // the props so that it's correctly styled
    const svgEl = svgElement(iconSVG)
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

  if (iconName) {
    return (
      <ApiIconRoot>
        <Icon>{iconName}</Icon>
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
    const defined = value !== undefined
    let status: UIInputStatus = { name: spec.name }
    // If the value isn't defined, use the default value
    // This is unfortunate duplication with the default value checks when initializing the
    // MUI managed input components. It might bee cleaner to initialize `inputValues` with
    // the default values. However, that breaks a bunch of stuff with persistence (e.g.,
    // if you modify one value, you get a cookie and then never get to see any default values
    // that get added/changed)
    if (spec.text) {
      status.text = { value: defined ? value : spec.text?.defaultValue }
    } else if (spec.bool) {
      status.bool = {
        value: (defined ? value : spec.bool.defaultValue) === true,
      }
    } else if (spec.hidden) {
      status.hidden = { value: spec.hidden.value }
    } else if (spec.choice) {
      status.choice = { value: defined ? value : spec.choice?.choices?.at(0) }
    }
    result.status!.inputs!.push(status)
  })

  return result
}

export async function updateButtonStatus(
  button: UIButton,
  inputValues: { [name: string]: any }
) {
  const toUpdate = buttonStatusWithInputs(button, inputValues)

  await tiltApiPut("uibuttons", "status", toUpdate)
}

export function ApiCancelButton(props: ApiButtonElementProps) {
  const { confirming, onClick, text, ...buttonProps } = props

  // Don't display the cancel confirmation button if the button
  // group's state isn't confirming
  if (!confirming) {
    return null
  }

  // To pass classes to a MUI component, it's necessary to use `classes`, instead of `className`
  const classes: Partial<ClassNameMap<ButtonClassKey>> = {
    root: "confirming rightButtonInGroup",
  }

  return (
    <ApiButtonElementRoot
      aria-label={`Cancel ${text}`}
      classes={classes}
      onClick={onClick}
      {...buttonProps}
    >
      <CloseSvg role="presentation" />
    </ApiButtonElementRoot>
  )
}

// The inner content of an ApiSubmitButton
export function ApiSubmitButtonContent(
  props: PropsWithChildren<{
    confirming: boolean
    displayButtonText: string
    iconName?: string
    iconSVG?: string
  }>
) {
  if (props.confirming) {
    return <ApiButtonLabel>{props.displayButtonText}</ApiButtonLabel>
  }

  if (props.children && props.children !== true) {
    return <>{props.children}</>
  }

  return (
    <>
      <ApiIcon iconName={props.iconName} iconSVG={props.iconSVG} />
      <ApiButtonLabel>{props.displayButtonText}</ApiButtonLabel>
    </>
  )
}

// For a toggle button that requires confirmation to trigger a UIButton's
// action, this component will render both the "submit" and the "confirm submit"
// HTML buttons. For keyboard navigation and accessibility, this component
// intentionally renders both buttons as the same element with different props.
// This makes sure that keyboard focus is moved to (or rather, stays on)
// the "confirm submit" button when the "submit" button is clicked. People
// using assistive tech like screenreaders will know they need to confirm.
// (Screenreaders should announce the "confirm submit" button to users because
// the `aria-label` changes when the "submit" button is clicked.)
export function ApiSubmitButton(
  props: PropsWithChildren<ApiButtonElementProps>
) {
  const {
    confirming,
    disabled,
    onClick,
    iconName,
    iconSVG,
    text,
    ...buttonProps
  } = props

  // Determine display text and accessible button label based on confirmation state
  const displayButtonText = confirming ? "Confirm" : text
  const ariaLabel = confirming ? `Confirm ${text}` : `Trigger ${text}`

  // To pass classes to a MUI component, it's necessary to use `classes`, instead of `className`
  const isConfirmingClass = confirming ? "confirming leftButtonInGroup" : ""
  const classes: Partial<ClassNameMap<ButtonClassKey>> = {
    root: isConfirmingClass,
  }

  // Note: button text is not included in analytics name since that can be user data
  return (
    <ApiButtonElementRoot
      aria-label={ariaLabel}
      classes={classes}
      disabled={disabled}
      onClick={onClick}
      {...buttonProps}
    >
      <ApiSubmitButtonContent
        confirming={confirming}
        displayButtonText={displayButtonText}
        iconName={iconName}
        iconSVG={iconSVG}
      >
        {props.children}
      </ApiSubmitButtonContent>
    </ApiButtonElementRoot>
  )
}

// Renders a UIButton.
// NB: The `Button` in `ApiButton` refers to a UIButton, not an html <button>.
// This can be confusing because each ApiButton consists of one or two <button>s:
// 1. A submit <button>, which fires the button's action.
// 2. Optionally, an options <button>, which allows the user to configure the
//    options used on submit.
export function ApiButton(props: PropsWithChildren<ApiButtonProps>) {
  const { className, uiButton, ...buttonProps } = props
  const buttonName = uiButton.metadata?.name || ""

  const [inputValues, setInputValues] = usePersistentState<{
    [name: string]: any
  }>(`apibutton-${buttonName}`, {})
  const { enqueueSnackbar } = useSnackbar()
  const pb = usePathBuilder()
  const { setError } = useHudErrorContext()

  const [loading, setLoading] = useState(false)
  const [confirming, setConfirming] = useState(false)

  // Reset the confirmation state when the button's name changes
  useLayoutEffect(() => setConfirming(false), [buttonName])

  const componentType = uiButton.spec?.location?.componentType as ApiButtonType
  const disabled = loading || uiButton.spec?.disabled || false
  const buttonText = uiButton.spec?.text || "Button"

  const onClick = async (e: React.MouseEvent<HTMLElement>) => {
    e.preventDefault()
    e.stopPropagation()

    if (uiButton.spec?.requiresConfirmation && !confirming) {
      setConfirming(true)
      return
    }

    if (confirming) {
      setConfirming(false)
    }

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

    // skip snackbar notifications for special buttons (e.g., disable, stop build)
    if (!buttonType(uiButton)) {
      const snackbarLogsLink =
        componentType === ApiButtonType.Global ? (
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
  }

  const submitButton = (
    <ApiSubmitButton
      text={buttonText}
      confirming={confirming}
      disabled={disabled}
      iconName={uiButton.spec?.iconName}
      iconSVG={uiButton.spec?.iconSVG}
      onClick={onClick}
      {...buttonProps}
    >
      {props.children}
    </ApiSubmitButton>
  )

  // show the options button if there are any non-hidden inputs
  const visibleInputs = uiButton.spec?.inputs?.filter((i) => !i.hidden) || []
  if (visibleInputs.length) {
    const setInputValue = (name: string, value: any) => {
      // Copy to a new object so that the reference changes to force a rerender.
      setInputValues({ ...inputValues, [name]: value })
    }
    const getInputValue = (name: string) => inputValues[name]

    return (
      <ApiButtonWithOptions
        className={className}
        submit={submitButton}
        uiButton={uiButton}
        setInputValue={setInputValue}
        getInputValue={getInputValue}
        aria-label={buttonText}
        // use-case-wise, it'd probably be better to leave the options button enabled
        // regardless of the submit button's state.
        // However, that's currently a low-impact difference, and this is a really
        // cheap way to ensure the styling matches.
        disabled={disabled}
        text={buttonText}
        {...buttonProps}
      />
    )
  } else {
    return (
      <ApiButtonRoot
        className={className}
        disableRipple={true}
        aria-label={buttonText}
        disabled={disabled}
      >
        {submitButton}
        <ApiCancelButton
          text={buttonText}
          confirming={confirming}
          disabled={disabled}
          onClick={() => setConfirming(false)}
          {...buttonProps}
        />
      </ApiButtonRoot>
    )
  }
}

function addButtonToSet(bs: ButtonSet, b: UIButton) {
  switch (buttonType(b)) {
    case UIBUTTON_TOGGLE_DISABLE_TYPE:
      bs.toggleDisable = b
      break
    case UIBUTTON_STOP_BUILD_TYPE:
      bs.stopBuild = b
      break
    default:
      bs.default.push(b)
      break
  }
}

export function buttonsForComponent(
  buttons: UIButton[] | undefined,
  componentType: ApiButtonType,
  componentID: string | undefined
): ButtonSet {
  let result = newButtonSet()
  if (!buttons) {
    return result
  }

  buttons.forEach((b) => {
    const buttonType = b.spec?.location?.componentType || ""
    const buttonID = b.spec?.location?.componentID || ""

    const buttonTypesMatch =
      buttonType.toUpperCase() === componentType.toUpperCase()
    const buttonIDsMatch = buttonID === componentID

    if (buttonTypesMatch && buttonIDsMatch) {
      addButtonToSet(result, b)
    }
  })

  return result
}

export function buttonsByComponent(
  buttons: UIButton[] | undefined
): Map<string, ButtonSet> {
  const result = new Map<string, ButtonSet>()

  if (buttons === undefined) {
    return result
  }

  buttons.forEach((b) => {
    const componentID = b.spec?.location?.componentID || ""

    // Disregard any buttons that aren't linked to a specific component or resource
    if (!componentID.length) {
      return
    }

    let buttonSet = result.get(componentID)
    if (!buttonSet) {
      buttonSet = newButtonSet()
      result.set(componentID, buttonSet)
    }

    addButtonToSet(buttonSet, b)
  })

  return result
}
