export class Hold {
  reason: string
  count: number = 0
  resources: string[] = []
  images: string[] = []

  constructor(waiting: Proto.v1alpha1UIResourceStateWaiting) {
    this.reason = waiting.reason ?? ""
    for (const ref of waiting.on ?? []) {
      this.count++
      if (ref.kind === "UIResource" && ref.name) {
        this.resources.push(ref.name)
      }
      if (ref.kind === "ImageMap" && ref.name) {
        this.images.push(ref.name)
      }
    }
  }
}
