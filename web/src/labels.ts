// Helper functions for working with labels and resource groups

import Features, { Flag } from "./feature"

export const UNLABELED_LABEL = "unlabeled"
export const TILTFILE_LABEL = "Tiltfile"

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
