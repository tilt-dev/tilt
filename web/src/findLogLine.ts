const LINE_ID_ATTR_NAME = "data-lineid"

export default function findLogLineID(
  el: HTMLElement | Node | null
): string | null {
  if (el === null) {
    return null
  }

  if (el instanceof HTMLElement && el.getAttribute(LINE_ID_ATTR_NAME)) {
    return el.getAttribute(LINE_ID_ATTR_NAME)
  } else if (el instanceof HTMLElement) {
    return findLogLineID(el.parentElement)
  } else if (el instanceof Node) {
    return findLogLineID(el.parentNode)
  }

  return null
}
