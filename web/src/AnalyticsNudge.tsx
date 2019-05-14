import React, { Component } from "react"
import "./AnalyticsNudge.scss"

class AnalyticsNudge extends Component {
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
          <button>Yes!</button> <button>Nope!</button>
        </div>
      </div>
    )
  }
}

export default AnalyticsNudge
