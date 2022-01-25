import { ApiButtonType, UIBUTTON_NAV_COMPONENT_ID } from "./ApiButton"
import { UIButton, UIInputSpec, UIInputStatus } from "./types"

export function textField(
  name: string,
  defaultValue?: string,
  placeholder?: string
): UIInputSpec {
  return {
    name: name,
    label: name,
    text: {
      defaultValue: defaultValue,
      placeholder: placeholder,
    },
  }
}

export function boolField(name: string, defaultValue?: boolean): UIInputSpec {
  return {
    name: name,
    label: name,
    bool: {
      defaultValue: defaultValue,
    },
  }
}

export function hiddenField(name: string, value: string): UIInputSpec {
  return {
    name: name,
    hidden: {
      value: value,
    },
  }
}

// TODO: Consider merging this test helper with `oneButton` in `testdata`
export function makeUIButton(args?: {
  name?: string
  inputSpecs?: UIInputSpec[]
  inputStatuses?: UIInputStatus[]
  requiresConfirmation?: boolean
  componentID?: string
}): UIButton {
  return {
    metadata: {
      name: args?.name ?? "TestButton",
    },
    spec: {
      text: "Click Me!",
      iconName: "flight_takeoff",
      inputs: args?.inputSpecs,
      location: {
        componentType: args?.componentID
          ? ApiButtonType.Resource
          : ApiButtonType.Global,
        componentID: args?.componentID ?? UIBUTTON_NAV_COMPONENT_ID,
      },
      requiresConfirmation: args?.requiresConfirmation,
    },
    status: {
      inputs: args?.inputStatuses,
    },
  }
}
