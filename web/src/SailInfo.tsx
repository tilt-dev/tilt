import React, { PureComponent } from "react"
import "./SailInfo.scss"

type SailProps = {
  sailEnabled: boolean
  sailUrl: string
}

class SailInfo extends PureComponent<SailProps> {
  render() {
    if (this.props.sailEnabled) {
      if (this.props.sailUrl) {
        return (
          <span className="SailInfo">
            <a href={this.props.sailUrl}>Share this view!</a>
          </span>
        )
      }

      return (
        <span className="SailInfo">
          <button type="button">Share me!</button>
        </span>
      )
    }

    return <span className="sail-url">&nbsp;</span>
  }
}

export default SailInfo
