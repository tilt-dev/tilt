import { flushSync } from "react-dom"

// Smoothly animates a layout change with the browser's View Transitions:
// the browser takes a picture of the page before and after the change, then
// moves and blends between them, so everything moves together as one motion.
//
// Mark elements that should move to their new place (rather than fade) with
// `transitionProps(key)`. The same key must mark the same thing before and
// after the change.
//
// Browsers without View Transitions (and people who prefer reduced motion)
// get the change instantly.

const KEY_ATTR = "data-vt"
const SCOPE_ATTR = "data-vt-scope"

// Uniform timing for every part of the change.
export const TRANSITION_DURATION_MS = 320
export const TRANSITION_EASING = "cubic-bezier(0.4, 0, 0.2, 1)"

export function transitionProps(key: string) {
  return { [KEY_ATTR]: key }
}

// Marks a scrolling container. Elements inside it only move if they start
// inside it; the rest just fade, so they don't draw over what's above it.
// (Running past the bottom is fine: the container ends at the window's edge.)
export const transitionScopeProps = { [SCOPE_ATTR]: "" }

export function prefersReducedMotion(): boolean {
  return !!window.matchMedia?.("(prefers-reduced-motion: reduce)").matches
}

// A valid, unique CSS name for a key (resource names can contain any
// characters).
export function transitionName(key: string): string {
  let hash = 0
  for (let i = 0; i < key.length; i++) {
    hash = (hash * 31 + key.charCodeAt(i)) | 0
  }
  const readable = key.replace(/[^a-zA-Z0-9_-]/g, "_")
  return `vt-${readable}-${(hash >>> 0).toString(36)}`
}

function isInView(el: HTMLElement): boolean {
  const rect = el.getBoundingClientRect()
  if (rect.width === 0 || rect.height === 0) {
    return false
  }
  const scope = el.closest(`[${SCOPE_ATTR}]`)
  if (!scope) {
    return true
  }
  const bounds = scope.getBoundingClientRect()
  return (
    rect.top >= bounds.top &&
    rect.top < bounds.bottom &&
    rect.left >= bounds.left &&
    rect.left < bounds.right
  )
}

function marked(root: HTMLElement): HTMLElement[] {
  return Array.from(root.querySelectorAll<HTMLElement>(`[${KEY_ATTR}]`))
}

function nameElements(root: HTMLElement) {
  marked(root).forEach((el) => {
    if (isInView(el)) {
      el.style.setProperty(
        "view-transition-name",
        transitionName(el.getAttribute(KEY_ATTR)!)
      )
    }
  })
}

function clearNames(root: HTMLElement) {
  marked(root).forEach((el) => el.style.removeProperty("view-transition-name"))
}

export function supportsViewTransitions(): boolean {
  return typeof (document as any).startViewTransition === "function"
}

// Runs `update` (a React state change) as a view transition.
export function runViewTransition(
  root: HTMLElement | null,
  update: () => void
) {
  if (!root || !supportsViewTransitions() || prefersReducedMotion()) {
    update()
    return
  }

  nameElements(root)
  const transition = (document as any).startViewTransition(() => {
    clearNames(root)
    flushSync(update)
    nameElements(root)
  })
  transition.finished.finally(() => clearNames(root))
}
