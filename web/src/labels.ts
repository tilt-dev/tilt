// Helper functions for working with labels and resource groups

import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
} from "@material-ui/core"
import styled from "styled-components"
import { ReactComponent as CaretSvg } from "./assets/svg/caret.svg"
import Features, { Flag } from "./feature"
import { Color, SizeUnit } from "./style-helpers"

// The generated type for labels is a generic object,
// when in reality, it is an object with string keys and values,
// so we can use a predicate function to type the labels object
// more precisely
type UILabelsGenerated = Pick<Proto.v1ObjectMeta, "labels">

interface UILabels extends UILabelsGenerated {
  labels: { [key: string]: string } | undefined
}

export type GroupByLabelView<T> = {
  labels: string[]
  labelsToResources: { [key: string]: T[] }
  tiltfile: T[]
  unlabeled: T[]
}

function isUILabels(
  labelsWrapper: UILabelsGenerated
): labelsWrapper is UILabels {
  return (
    labelsWrapper.labels === undefined ||
    typeof labelsWrapper.labels === "object"
  )
}

export function asUILabels(labels: UILabelsGenerated): UILabels {
  if (isUILabels(labels)) {
    return labels
  }

  return { labels: undefined } as UILabels
}

// Following k8s practices, we treat labels with prefixes as
// added by external tooling and not relevant to the user
// k8s practices outline that automated tooling prefix
export function getUILabels({ labels }: UILabels): string[] {
  if (!labels) {
    return []
  }

  return Object.keys(labels)
    .filter((labelKey) => {
      const labelHasPrefix = labelKey.includes("/")
      return !labelHasPrefix
    })
    .map((labelKey) => labels[labelKey])
}

// Order labels alphabetically A - Z
export function orderLabels(labels: string[]) {
  return [...labels].sort((a, b) => a.localeCompare(b))
}

// This helper function takes a template type for the resources
// and a label accessor function
export function resourcesHaveLabels<T>(
  features: Features,
  resources: T[] | undefined,
  getLabels: (resource: T) => string[]
): boolean {
  // Labels on resources are ignored if feature is not enabled
  if (!features.isEnabled(Flag.Labels)) {
    return false
  }

  if (resources === undefined) {
    return false
  }

  return resources.some((r) => getLabels(r).length > 0)
}

// Shared resource grouping styles
// TODO: Consider moving these to a resource grouping specific file
export const Group = styled(Accordion)`
  &.MuiPaper-root {
    background-color: unset;
  }

  &.MuiPaper-elevation1 {
    box-shadow: unset;
  }

  &.MuiAccordion-root,
  &.MuiAccordion-root.Mui-expanded {
    margin: ${SizeUnit(1 / 3)} ${SizeUnit(1 / 2)};
  }
`

export const SummaryIcon = styled(CaretSvg)`
  flex-shrink: 0;
  padding: ${SizeUnit(1 / 4)};
  transition: transform 300ms cubic-bezier(0.4, 0, 0.2, 1) 0ms; /* Copied from MUI accordion */

  .fillStd {
    fill: ${Color.grayLight};
  }
`

export const GroupName = styled.span`
  margin-right: auto;
  overflow: hidden;
  text-overflow: ellipsis;
  width: 100%;
`

export const GroupSummary = styled(AccordionSummary)`
  &.MuiAccordionSummary-root,
  &.MuiAccordionSummary-root.Mui-expanded {
    min-height: unset;
    padding: unset;
  }

  .MuiAccordionSummary-content {
    align-items: center;
    box-sizing: border-box;
    color: ${Color.white};
    display: flex;
    margin: 0;
    padding: ${SizeUnit(1 / 8)};
    width: 100%;

    &.Mui-expanded {
      margin: 0;

      ${SummaryIcon} {
        transform: rotate(90deg);
      }
    }
  }
`

export const GroupDetails = styled(AccordionDetails)`
  &.MuiAccordionDetails-root {
    display: unset;
    padding: unset;
  }
`
