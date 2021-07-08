// Helper functions for working with labels

type UILabels = Pick<Proto.v1ObjectMeta, "labels">

// TODO (LT): Add a lil' explanation here about k8s use of labels
// and why we filter out labels with prefixes
export function getUiLabels({ labels }: UILabels) {
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
