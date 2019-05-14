import React, { Component } from "react"
import "./AnalyticsNudge.scss"

class AnalyticsNudge extends Component {
  static analyticsOpt(optIn: boolean) {
    let url = `//${window.location.host}/api/analytics/opt`

    let payload = { opt: optIn ? "opt-in" : "opt-out" }

    let j = JSON.stringify(payload)
    console.log(j)

    fetch(url, {
      method: "post",
      body: JSON.stringify(payload),
    })
  }
  render() {
    return (
      <div className="AnalyticsNudge" key="AnalyticsNudge">
        <div>
          Congrats on your first Tilt resource 🎉 Help us help you: may we
          collect anonymized data on your usage? (Read more{" "}
          <a href="https://github.com/windmilleng/tilt#privacy" target="_blank">
            here
          </a>
          .)
        </div>
        <div className="AnalyticsNudge-buttons">
          <button onClick={() => AnalyticsNudge.analyticsOpt(true)}>
            Yes!
          </button>{" "}
          <button onClick={() => AnalyticsNudge.analyticsOpt(false)}>
            Nope!
          </button>
        </div>
      </div>
    )
  }
}

export default AnalyticsNudge
