import React, { Component } from "react"
import "./AnalyticsNudge.scss"

type AnalyticsNudgeProps = {
  needsNudge: boolean
}

type AnalyticsNudgeState = {
  requestMade: boolean
  responseCode: number
  dismissed: boolean
}

// constructor for defaults?

class AnalyticsNudge extends Component<
  AnalyticsNudgeProps,
  AnalyticsNudgeState
> {
  shouldShow(): boolean {
    return (
      this.props.needsNudge ||
      (this.state && this.state.requestMade && !this.state.dismissed)
    )
  }
  analyticsOpt(optIn: boolean) {
    let url = `//${window.location.host}/api/analytics_opt`

    let payload = { opt: optIn ? "opt-in" : "opt-out" }

    this.setState({ requestMade: true })

    fetch(url, {
      method: "post",
      body: JSON.stringify(payload),
    }).then((response: Response) => {
      this.setState({
        responseCode: response.status,
      })

      // after 3s, dismiss the nudge
      setTimeout(() => {
        this.setState({ dismissed: true })
      }, 3000)
    })
  }

  messageElem(): JSX.Element {
    if (this.state && this.state.responseCode) {
      if (this.state.responseCode == 200) {
        // Successfully called opt endpt.
        return <span>Cool, got it! 👍</span>
      } else {
        return (
          // error calling the opt endpt.
          <span>
            Oh no, something went wrong!{" "}
            <a href="https://tilt.dev/contact" target="_blank">
              Get in touch
            </a>
            .
          </span>
        )
      }
    }

    if (this.state && this.state.requestMade) {
      // request in progress
      return <span>Okay, we'll inform the robots...</span>
    }
    return (
      <div>
        <span>
          Congrats on your first Tilt resource 🎉 Opt into analytics? (Read more{" "}
          <a href="https://github.com/windmilleng/tilt#privacy" target="_blank">
            here
          </a>
          .)&nbsp;
        </span>
        <span className="AnalyticsNudge-buttons">
          <button onClick={() => this.analyticsOpt(true)}>Yes!</button>
          <button onClick={() => this.analyticsOpt(false)}>Nope!</button>
        </span>
      </div>
    )
  }
  render() {
    let classes = ["AnalyticsNudge"]
    if (this.shouldShow()) {
      // or if already visible...
      classes.push("is-visible")
    }

    return <div className={classes.join(" ")}>{this.messageElem()}</div>
  }
}

export default AnalyticsNudge
