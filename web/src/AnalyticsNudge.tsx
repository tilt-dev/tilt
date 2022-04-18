import React, { Component } from "react"
import "./AnalyticsNudge.scss"
import { linkToTiltDocs, TiltDocsPage } from "./constants"

export const NUDGE_TIMEOUT_MS = 15000
const nudgeElem = (): JSX.Element => {
  return (
    <p>
      Welcome to Tilt! We collect anonymized usage data to help us improve. Is
      that OK? (
      <a
        href={linkToTiltDocs(TiltDocsPage.TelemetryFaq)}
        target="_blank"
        rel="noopener noreferrer"
      >
        Read more
      </a>
      )
    </p>
  )
}
const successOptInElem = (): JSX.Element => {
  return (
    <p data-testid="optin-success">
      Thanks for helping us improve Tilt for you and everyone! (You can change
      your mind by running <code>tilt analytics opt out</code> in your
      terminal.)
    </p>
  )
}
const successOptOutElem = (): JSX.Element => {
  return (
    <p data-testid="optout-success">
      Okay, opting you out of telemetry. If you change your mind, you can run{" "}
      <code>tilt analytics opt in</code> in your terminal.
    </p>
  )
}
const errorElem = (respBody: string): JSX.Element => {
  return (
    <div role="alert">
      <p>Oh no, something went wrong! Request failed with:</p>
      <pre>{respBody}</pre>
      <p>
        <a
          href="https://tilt.dev/contact"
          target="_blank"
          rel="noopener noreferrer"
        >
          contact us
        </a>
      </p>
    </div>
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
        }, NUDGE_TIMEOUT_MS)
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
      return <p data-testid="opt-loading">Okay, making it so...</p>
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
    if (!this.shouldShow()) {
      return null
    }

    return (
      <aside
        aria-label="Tilt analytics options"
        className="AnalyticsNudge is-visible"
      >
        {this.messageElem()}
      </aside>
    )
  }
}

export default AnalyticsNudge
