import React, { PureComponent } from "react"

type PreviewProps = {
  endpoint: string
}

class PreviewPane extends PureComponent<PreviewProps> {
  render() {
    return <iframe src={this.props.endpoint} />
  }
}

export default PreviewPane
