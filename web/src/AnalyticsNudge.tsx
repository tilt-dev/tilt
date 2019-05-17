import React, { Component } from "react"
import "./AnalyticsNudge.scss"

const nudgeTimeoutMs = 3000 // 3 seconds

type AnalyticsNudgeProps = {
  needsNudge: boolean
}

type AnalyticsNudgeState = {
  requestMade: boolean
  optIn: boolean
  responseCode: number
  responseBody: string
  dismissed: boolean
}

class AnalyticsNudge extends Component<
  AnalyticsNudgeProps,
  AnalyticsNudgeState
> {
  constructor(props: AnalyticsNudgeProps) {
    super(props)

    this.state = {
      requestMade: false,
      optIn: false,
      responseCode: 0,
      responseBody: "",
      dismissed: false,
    }
  }

  shouldShow(): boolean {
    return (
      this.props.needsNudge || (this.state.requestMade && !this.state.dismissed)
    )
  }

  analyticsOpt(optIn: boolean) {
    let url = `//${window.location.host}/api/analytics_opt`

    let payload = { opt: optIn ? "opt-in" : "opt-out" }

    this.setState({ requestMade: true, optIn: optIn })

    fetch(url, {
      method: "post",
      body: JSON.stringify(payload),
    }).then((response: Response) => {
      response.text().then((body: string) => {
        this.setState({
          responseCode: response.status,
          responseBody: body,
        })
      })

      if (response.status == 200) {
        // if we successfully recorded the choice, dismiss the nudge after 3s
        setTimeout(() => {
          this.setState({ dismissed: true })
        }, nudgeTimeoutMs)
      }
    })
  }

  dismiss() {
    this.setState({ dismissed: true })
  }

  messageElem(): JSX.Element {
    if (this.state.responseCode) {
      if (this.state.responseCode == 200) {
        // Successfully called opt endpt.
        if (this.state.optIn) {
          // User opted in
          return <span>Thanks for opting in! [copy tbd]</span>
        }

        // User opted out
        return <span>Thanks for opting out! [copy tbd]</span>
      } else {
        return (
          // Error calling the opt endpt.
          <div>
            <span>
              Oh no, something went wrong! Request failed with:
              <div className="AnalyticsNudge-err">
                <span>{this.state.responseBody}</span>
              </div>
              <a href="https://tilt.dev/contact" target="_blank">
                Contact us
              </a>
              .&nbsp;
            </span>
            <span className="AnalyticsNudge-buttons">
              <button onClick={() => this.dismiss()}>Dismiss</button>
            </span>
          </div>
        )
      }
    }

    if (this.state.requestMade) {
      // Request in progress
      return <span>Okay, we'll inform the robots...</span>
    }
    return (
      <div>
        <span>
          Congrats on your first Tilt resource 🎉 Opt into analytics? (Read more{" "}
          <a
            href="https://github.com/windmilleng/tilt#telemetry-and-privacy"
            target="_blank"
          >
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
