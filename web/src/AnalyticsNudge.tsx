import React, { PureComponent } from "react"
import "./AnalyticsNudge.scss"
import { ResourceView } from "./types"

type AnalyticsNudgeProps = {
  needsNudge: boolean
}

class AnalyticsNudge extends PureComponent<AnalyticsNudgeProps> {
  static analyticsOpt(optIn: boolean) {
    let url = `//${window.location.host}/api/analytics_opt`

    let payload = { opt: optIn ? "opt-in" : "opt-out" }

    fetch(url, {
      method: "post",
      body: JSON.stringify(payload),
    }).then(function(response) {
      console.log("got response -->", response.status) // returns 200
    })
  }
  render() {
    let classes = ["AnalyticsNudge"]
    if (this.props.needsNudge) {
      // or if  already visible...
      classes.push("is-visible")
    }

    return (
      <div className={classes.join(" ")}>
        <span>
          Congrats on your first Tilt resource 🎉 Opt into analytics? (Read more{" "}
          <a href="https://github.com/windmilleng/tilt#privacy" target="_blank">
            here
          </a>
          .)&nbsp;
        </span>
        <span className="AnalyticsNudge-buttons">
          <button onClick={() => AnalyticsNudge.analyticsOpt(true)}>
            Yes!
          </button>
          <button onClick={() => AnalyticsNudge.analyticsOpt(false)}>
            Nope!
          </button>
        </span>
      </div>
    )
  }
}

export default AnalyticsNudge
