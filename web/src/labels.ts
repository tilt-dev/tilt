// Helper functions for working with labels and resource groups

import Features, { Flag } from "./feature"
import { UIResource } from "./types"

export const UNLABELED_LABEL = "unlabeled"
export const TILTFILE_LABEL = "Tiltfile"

export type GroupByLabelView<T> = {
  labels: string[]
  labelsToResources: { [key: string]: T[] }
  tiltfile: T[]
  unlabeled: T[]
}

// Hierarchical tree node for nested label groups
export type GroupTreeNode<T> = {
  name: string // Display name (e.g., "apis")
  path: string // Full path (e.g., "env-1.apis")
  resources: T[] // Direct resources in this group
  children: GroupTreeNode<T>[] // Child groups
  aggregatedResources: T[] // All descendants (for status counts)
}

// Replaces GroupByLabelView for hierarchical labels
export type HierarchicalGroupView<T> = {
  roots: GroupTreeNode<T>[] // Top-level groups
  tiltfile: T[]
  unlabeled: T[]
  allGroupPaths: string[] // For ResourceGroupsContext
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
type UILabelsGenerated = Pick<Proto.v1ObjectMeta, "labels">

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

// Known K8s prefixes to filter out (system labels, not user-defined)
const K8S_LABEL_PREFIXES = [
  "kubernetes.io/",
  "app.kubernetes.io/",
  "helm.sh/",
  "k8s.io/",
]

// Following k8s practices, we treat labels with K8s system prefixes as
// added by external tooling and not relevant to the user.
// Custom hierarchical labels use "." as separator (e.g., "env-1.apis").
export function getResourceLabels(resource: UIResource): string[] {
  // Safely cast and extract labels from a resource
  const { labels: labelsMap } = asUILabels({
    labels: resource.metadata?.labels,
  })
  if (!labelsMap) {
    return []
  }

  // Return the labels in the form of a list, not a map
  // Filter out K8s system labels, but allow custom hierarchical labels
  return Object.keys(labelsMap)
    .filter((label) => {
      return !K8S_LABEL_PREFIXES.some((prefix) => label.startsWith(prefix))
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

// Build a hierarchical tree from resources with labels that may contain "."
export function buildGroupTree<T>(
  resources: T[],
  getLabels: (resource: T) => string[],
  isTiltfile: (resource: T) => boolean
): HierarchicalGroupView<T> {
  const allNodes = new Map<string, GroupTreeNode<T>>()
  const rootMap = new Map<string, GroupTreeNode<T>>()
  const unlabeled: T[] = []
  const tiltfile: T[] = []

  // Helper to get or create node at path
  function getOrCreateNode(path: string): GroupTreeNode<T> {
    if (allNodes.has(path)) return allNodes.get(path)!

    const parts = path.split(".")
    const name = parts[parts.length - 1]
    const node: GroupTreeNode<T> = {
      name,
      path,
      resources: [],
      children: [],
      aggregatedResources: [],
    }
    allNodes.set(path, node)

    if (parts.length === 1) {
      rootMap.set(path, node)
    } else {
      const parentPath = parts.slice(0, -1).join(".")
      const parent = getOrCreateNode(parentPath)
      // Insert in sorted order
      const idx = parent.children.findIndex(
        (c) => c.name.localeCompare(name) > 0
      )
      if (idx === -1) parent.children.push(node)
      else parent.children.splice(idx, 0, node)
    }
    return node
  }

  // Process resources
  resources.forEach((resource) => {
    if (isTiltfile(resource)) {
      tiltfile.push(resource)
      return
    }

    const labels = getLabels(resource)
    if (labels.length === 0) {
      unlabeled.push(resource)
      return
    }

    labels.forEach((label) => getOrCreateNode(label).resources.push(resource))
  })

  // Compute aggregated resources (post-order traversal)
  function computeAggregated(node: GroupTreeNode<T>): T[] {
    const aggregated = [...node.resources]
    node.children.forEach((child) =>
      aggregated.push(...computeAggregated(child))
    )
    node.aggregatedResources = Array.from(new Set(aggregated)) // Dedupe
    return node.aggregatedResources
  }

  const roots = Array.from(rootMap.values()).sort((a, b) =>
    a.name.localeCompare(b.name)
  )
  roots.forEach(computeAggregated)

  return {
    roots,
    tiltfile,
    unlabeled,
    allGroupPaths: Array.from(allNodes.keys()).sort(),
  }
}
