// This promise resolves when all in-flight promises have resolved.
// See: https://github.com/facebook/jest/issues/2157#issuecomment-279171856

export function flushPromises() {
  return new Promise((resolve) => setImmediate(resolve))
}
