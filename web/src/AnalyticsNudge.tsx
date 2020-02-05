import React, { Component } from "react"
import "./AnalyticsNudge.scss"

const nudgeTimeoutMs = 15000
const nudgeElem = (): JSX.Element => {
  return (
    <p>
      Welcome to Tilt! We collect anonymized usage data to help us improve. Is
      that OK? (
      <a
        href="https://docs.tilt.dev/telemetry_faq.html"
        target="_blank"
        rel="noopener noreferrer"
      >
        Read more
      </a>
      )
    </p>
  )
}
const reqInProgMsg = "Okay, making it so..."
const successOptInElem = (): JSX.Element => {
  return (
    <p>
      Thanks for helping us improve Tilt for you and everyone! (You can change
      your mind by running <pre>tilt analytics opt out</pre> in your terminal.)
    </p>
  )
}
const successOptOutElem = (): JSX.Element => {
  return (
    <p>
      Okay, opting you out of telemetry. If you change your mind, you can run{" "}
      <pre>tilt analytics opt in</pre> in your terminal.
    </p>
  )
}
const errorElem = (respBody: string): JSX.Element => {
  return (
    <p>
      Oh no, something went wrong! Request failed with: <pre>{respBody}</pre> (
      <a
        href="https://tilt.dev/contact"
        target="_blank"
        rel="noopener noreferrer"
      >
        contact us
      </a>
      )
    </p>
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
      (this.props.needsNudge && !this.state.dismissed) ||
      (!this.state.dismissed && this.state.requestMade)
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

      if (response.status === 200) {
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
      if (this.state.responseCode === 200) {
        // Successfully called opt endpt.
        if (this.state.optIn) {
          // User opted in
          return (
            <>
              {successOptInElem()}
              <span>
                <button
                  className="AnalyticsNudge-button"
                  onClick={() => this.dismiss()}
                >
                  Dismiss
                </button>
              </span>
            </>
          )
        }
        // User opted out
        return (
          <>
            {successOptOutElem()}
            <span>
              <button
                className="AnalyticsNudge-button"
                onClick={() => this.dismiss()}
              >
                Dismiss
              </button>
            </span>
          </>
        )
      } else {
        return (
          // Error calling the opt endpt.
          <>
            {errorElem(this.state.responseBody)}
            <span>
              <button
                className="AnalyticsNudge-button"
                onClick={() => this.dismiss()}
              >
                Dismiss
              </button>
            </span>
          </>
        )
      }
    }

    if (this.state.requestMade) {
      // Request in progress
      return <p>{reqInProgMsg}</p>
    }
    return (
      <>
        {nudgeElem()}
        <span>
          <button
            className="AnalyticsNudge-button"
            onClick={() => this.analyticsOpt(false)}
          >
            Nope
          </button>
          <button
            className="AnalyticsNudge-button opt-in"
            onClick={() => this.analyticsOpt(true)}
          >
            I'm in
          </button>
        </span>
      </>
    )
  }
  render() {
    let classes = ["AnalyticsNudge"]
    if (this.shouldShow()) {
      classes.push("is-visible")
    }

    return <section className={classes.join(" ")}>{this.messageElem()}</section>
  }
}

export default AnalyticsNudge
