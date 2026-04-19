// Helper functions for working with labels and resource groups

import Features, { Flag } from "./feature"
import { UIResource } from "./types"
import type { ObjectMeta } from "./types"

export const UNLABELED_LABEL = "unlabeled"
export const TILTFILE_LABEL = "Tiltfile"

export type GroupByLabelView<T> = {
  labels: string[]
  labelsToResources: { [key: string]: T[] }
  tiltfile: T[]
  unlabeled: T[]
}

/**
 * The generated type for labels is a generic object,
 * but in reality, it's an object with string keys and values.
 * (This is a little bit of typescript gymnastics.)
 *
 * `isUILabels` is a type predicate function that asserts
 * whether or not its input is the `UILabels` type
 *
 * `asUILabels` safely casts its input into a `UILabels` type
 */
type UILabelsGenerated = Pick<ObjectMeta, "labels">

interface UILabels extends UILabelsGenerated {
  labels: { [key: string]: string } | undefined
}

function isUILabels(
  labelsWrapper: UILabelsGenerated
): labelsWrapper is UILabels {
  return (
    labelsWrapper.labels === undefined ||
    typeof labelsWrapper.labels === "object"
  )
}

function asUILabels(labels: UILabelsGenerated): UILabels {
  if (isUILabels(labels)) {
    return labels
  }

  return { labels: undefined } as UILabels
}

// Following k8s practices, we treat labels with prefixes as
// added by external tooling and not relevant to the user
export function getResourceLabels(resource: UIResource): string[] {
  // Safely cast and extract labels from a resource
  const { labels: labelsMap } = asUILabels({
    labels: resource.metadata?.labels,
  })
  if (!labelsMap) {
    return []
  }

  // Return the labels in the form of a list, not a map
  return Object.keys(labelsMap)
    .filter((label) => {
      const labelHasPrefix = label.includes("/")
      return !labelHasPrefix
    })
    .map((label) => labelsMap[label])
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

// Namespace grouping support for Kubernetes resources
const DEFAULT_NAMESPACE = "default"
const UNGROUPED_NAMESPACE = "ungrouped"

export type GroupByNamespaceView<T> = {
  namespaces: string[]
  namespacesToResources: { [key: string]: T[] }
  ungrouped: T[]
}

/**
 * Extract namespace from a K8s resource
 * Resources with namespace info will be grouped by namespace
 */
export function getResourceNamespace(resource: UIResource): string | null {
  const k8sInfo = resource.status?.k8sResourceInfo
  if (!k8sInfo || !k8sInfo.namespace) {
    return null
  }
  return k8sInfo.namespace
}

/**
 * Check if resources have namespace information
 * This allows conditional grouping by namespace
 */
export function resourcesHaveNamespaces<T>(
  resources: T[] | undefined,
  getNamespace: (resource: T) => string | null
): boolean {
  if (resources === undefined) {
    return false
  }

  return resources.some((r) => getNamespace(r) !== null)
}

/**
 * Group resources by their Kubernetes namespace
 * Resources without namespace info are placed in "ungrouped"
 */
export function groupResourcesByNamespace<T>(
  resources: T[],
  getNamespace: (resource: T) => string | null
): GroupByNamespaceView<T> {
  const namespacesToResources: { [key: string]: T[] } = {}
  const ungrouped: T[] = []
  const namespacesSet = new Set<string>()

  resources.forEach((resource) => {
    const namespace = getNamespace(resource)
    if (namespace) {
      if (!namespacesToResources[namespace]) {
        namespacesToResources[namespace] = []
      }
      namespacesToResources[namespace].push(resource)
      namespacesSet.add(namespace)
    } else {
      ungrouped.push(resource)
    }
  })

  // Sort namespaces alphabetically, with "default" first if present
  const namespaces = Array.from(namespacesSet).sort((a, b) => {
    if (a === DEFAULT_NAMESPACE) return -1
    if (b === DEFAULT_NAMESPACE) return 1
    return a.localeCompare(b)
  })

  return {
    namespaces,
    namespacesToResources,
    ungrouped,
  }
}
