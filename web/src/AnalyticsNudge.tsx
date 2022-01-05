import React, { Component } from "react"
import styled from "styled-components"
import { linkToTiltDocs, TiltDocsPage } from "./constants"
import { Color, SizeUnit } from "./style-helpers"

const AnalyticsNudgeRoot = styled.section`
  color: ${Color.grayDarkest};
  position: fixed;
  width: 100%;
  box-sizing: border-box;
  padding: ${SizeUnit(0.5)};
  justify-content: space-between;
  bottom: 0;
  background-color: ${Color.gray};
  color: ${Color.white};
  z-index: $z-analyticsNudge;
  display: none;

  &.is-visible {
    display: flex;
  }
`

const AnalyticsNudgeButtons = styled.span`
  display: flex;
`

const AnalyticsNudgeButton = styled.button`
  align-content: center;
  background-color: ${Color.grayDark};
  color: ${Color.grayLightest};
  border: 1px solid ${Color.grayLight};
  padding: ${SizeUnit(0.5)};
  margin-left: ${SizeUnit(0.5)};
`

const AnalyticsNudgeButtonConfirm = styled(AnalyticsNudgeButton)`
  color: ${Color.green};
`

/**
 * Clean-up of Analytics nudge:
 * - (done) location may inconvience people and illicit a quick NOPE response
 * - (done) wording
 * - a11y issue (lizz will check)
 * - buttons and colors don't vibe with rest of design (han will refine)
 * - refactor with styled-components
 * - (skip) use existing button component
 * - (skip) refactor to simplify code
 */

const nudgeTimeoutMs = 15000
const DefaultMessage = (): JSX.Element => {
  return (
    <p>
      Help us improve Tilt by letting us collect anonymized usage data! We
      respect your privacy. You can{" "}
      <a
        href={linkToTiltDocs(TiltDocsPage.TelemetryFaq)}
        target="_blank"
        rel="noopener noreferrer"
      >
        learn more
      </a>{" "}
      about exactly what data we collect and how we use it.
    </p>
  )
}
const reqInProgMsg = "…updating…"
const OptInSuccess = (): JSX.Element => {
  return (
    <p>
      Thank you so much! ✨ If you change your mind, you can opt out via command
      line: <code>tilt analytics opt out</code>.
    </p>
  )
}
const OptOutSuccess = (): JSX.Element => {
  return (
    <p>
      Opted out of telemetry. If you change your mind, you can opt in via
      command line: <code>tilt analytics opt in</code>.
    </p>
  )
}
const ErrorMessage = (props: { respBody: string }): JSX.Element => {
  return (
    <p>
      Oh no, something went wrong! Request failed with:{" "}
      <code>{props.respBody}</code> (
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
              <OptInSuccess />
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
            <OptOutSuccess />
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
            <ErrorMessage respBody={this.state.responseBody} />
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
        <DefaultMessage />
        <AnalyticsNudgeButtons>
          <AnalyticsNudgeButton
            className="AnalyticsNudge-button"
            onClick={() => this.analyticsOpt(false)}
          >
            No Thanks
          </AnalyticsNudgeButton>
          <AnalyticsNudgeButtonConfirm
            className="AnalyticsNudge-button"
            onClick={() => this.analyticsOpt(true)}
          >
            I'm In!
          </AnalyticsNudgeButtonConfirm>
        </AnalyticsNudgeButtons>
      </>
    )
  }
  render() {
    let classes = ["AnalyticsNudge"]
    if (this.shouldShow()) {
      classes.push("is-visible")
    }

    return (
      <AnalyticsNudgeRoot role="alert" className={classes.join(" ")}>
        {this.messageElem()}
      </AnalyticsNudgeRoot>
    )
  }
}

export default AnalyticsNudge
