import React, { Component } from "react"
import "./AnalyticsNudge.scss"

const nudgeTimeoutMs = 3000 // 3 seconds
let nudgeElem = (): JSX.Element => {
  return (
    <span>
      Welcome to Tilt! To better support you, may we record anonymized data
      about your usage? (
      <a
        href="https://github.com/windmilleng/tilt#telemetry-and-privacy"
        target="_blank"
      >
        read more
      </a>
      .)&nbsp;
    </span>
  )
}
const reqInProgMsg = "request in prog"
const successOptInMsg = "yay you opted in"
const successOptOutMsg = "whelp you opted out"
let errorElem = (respBody: string): JSX.Element => {
  return (
    <span>
      Oh no, something went wrong! Request failed with:
      <div className="AnalyticsNudge-err">
        <span>{respBody}</span>
      </div>
      <a href="https://tilt.dev/contact" target="_blank">
        Contact us
      </a>
      .&nbsp;
    </span>
  )
}

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
          return <span>{successOptInMsg}</span>
        }

        // User opted out
        return <span>{successOptOutMsg}</span>
      } else {
        return (
          // Error calling the opt endpt.
          <div>
            {errorElem(this.state.responseBody)}
            <span className="AnalyticsNudge-buttons">
              <button onClick={() => this.dismiss()}>Dismiss</button>
            </span>
          </div>
        )
      }
    }

    if (this.state.requestMade) {
      // Request in progress
      return <span>{reqInProgMsg}</span>
    }
    return (
      <div>
        {nudgeElem()}
        <span className="AnalyticsNudge-buttons">
          <button onClick={() => this.analyticsOpt(true)}>Yes</button>
          <button onClick={() => this.analyticsOpt(false)}>No</button>
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
