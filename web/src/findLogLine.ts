const LINE_ID_ATTR_NAME = "data-lineid"

export default function findLogLineID(node: Node | null): string | null {
  if (node === null) {
    return null
  }

  let el = node as HTMLElement

  if (
    typeof el.getAttribute === "function" &&
    el.getAttribute(LINE_ID_ATTR_NAME)
  ) {
    return el.getAttribute(LINE_ID_ATTR_NAME)
  } else if (el.parentNode) {
    return findLogLineID(el.parentElement)
  } else if (node instanceof Node) {
    return findLogLineID((node as Node).parentNode)
  }

  return null
}
