const LINE_ID_ATTR_NAME = "data-lineid"

export default function findLogLineID(
  el: HTMLElement | Node | null
): string | null {
  if (el === null) {
    return null
  }

  if ((el as HTMLElement).getAttribute(LINE_ID_ATTR_NAME)) {
    return (el as HTMLElement).getAttribute(LINE_ID_ATTR_NAME)
  } else if ((el as HTMLElement).parentNode) {
    return findLogLineID((el as HTMLElement).parentElement)
  } else if (el instanceof Node) {
    return findLogLineID((el as Node).parentNode)
  }

  return null
}
