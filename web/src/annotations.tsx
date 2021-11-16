// get annotations from an api object w/ a reasonable type
//
// (our codegen generates the `annotations` field with type `object`, which is not very useful)
export function annotations(obj: {
  metadata?: Proto.v1ObjectMeta
}): { [name: string]: string } {
  return (obj.metadata?.annotations ?? {}) as { [name: string]: string }
}
