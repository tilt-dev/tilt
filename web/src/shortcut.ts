// if the user is typing in one of these elements, those key-presses shouldn't trigger shortcuts
const ignoredTags = ["input", "textarea"]

export function isTargetEditable(e: KeyboardEvent): boolean {
  // NB: this disables *all* custom shortcuts while in an editable field, including, e.g. ctrl+bksp.
  // We could return false if e.KeyCtrl, but then ctrl+v could trigger a 'v' shortcut, which is bad.
  // We can worry about those issues if/when they come up.
  return (
    e.target instanceof Element &&
    ignoredTags.includes(e.target.tagName.toLowerCase())
  )
}
