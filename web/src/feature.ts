// This Features wrapper behaves differently than the one in Go.
// Checking for features that don't exist is *not* an error here.
// This is important because when the React app starts,
// it starts with an empty state and there won't be _any_ feature flags
// until the first engine state comes in over the Websocket.
export default class Features {
  private flags: { [featureFlag: string]: boolean }

  constructor(flags: { [featureFlag: string]: boolean }) {
    this.flags = flags
  }

  public isEnabled(flag: string): boolean {
    if (this.flags.hasOwnProperty(flag)) {
      return this.flags[flag]
    }
    return false
  }
}
