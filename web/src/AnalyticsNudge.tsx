import React, { Component } from "react"
import "./AnalyticsNudge.scss"

const nudgeTimeoutMs = 5000 // 5 seconds
const nudgeElem = (): JSX.Element => {
  return (
    <span>
      Welcome to Tilt! Usage data helps us improve; will you contribute? (
      <a
        href="https://github.com/windmilleng/tilt#telemetry-and-privacy"
        target="_blank"
      >
        Read more
      </a>
      .)&nbsp;
    </span>
  )
}
const reqInProgMsg = "Okay, making it so..."
const successOptInElem = (): JSX.Element => {
  return (
    <span>
      Thanks for helping us improve Tilt for you and everyone! (You can change
      your mind by running <pre>tilt analytics opt out</pre> in your
      terminal.)&nbsp;
    </span>
  )
}
const successOptOutElem = (): JSX.Element => {
  return (
    <span>
      Okay, opting you out of telemetry. If you change your mind, you can run{" "}
      <pre>tilt analytics opt in</pre> in your terminal.&nbsp;
    </span>
  )
}
const errorElem = (respBody: string): JSX.Element => {
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
        // if we successfully recorded the choice, dismiss the nudge after a few seconds
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
          return (
            <div>
              {successOptInElem()}
              <span className="AnalyticsNudge-buttons">
                <button onClick={() => this.dismiss()}>Dismiss</button>
              </span>
            </div>
          )
        }
        // User opted out
        return (
          <div>
            {successOptOutElem()}
            <span className="AnalyticsNudge-buttons">
              <button onClick={() => this.dismiss()}>Dismiss</button>
            </span>
          </div>
        )
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
          <button onClick={() => this.analyticsOpt(true)}>
            Yes, I want to help!
          </button>
          <button onClick={() => this.analyticsOpt(false)}>
            No, I'd rather not.
          </button>
        </span>
      </div>
    )
  }
  render() {
    let classes = ["AnalyticsNudge"]
    if (this.shouldShow()) {
      classes.push("is-visible")
    }

    return <div className={classes.join(" ")}>{this.messageElem()}</div>
  }
}

export default AnalyticsNudge
