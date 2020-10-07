import React, { Component } from "react"
import { History } from "history"
import { useHistory } from "react-router"

type Props = {
  logUrl: string
  alertsUrl: string
  facetsUrl: string | null
  history: History
}

/**
 * Sets up keyboard shortcuts that depend on the state of the secondary nav.
 */
class SecondaryNavKeyboardShortcuts extends Component<Props> {
  constructor(props: Props) {
    super(props)
    this.onKeydown = this.onKeydown.bind(this)
  }

  componentDidMount() {
    document.body.addEventListener("keydown", this.onKeydown)
  }

  componentWillUnmount() {
    document.body.removeEventListener("keydown", this.onKeydown)
  }

  onKeydown(e: KeyboardEvent) {
    let key = e.key
    if (e.metaKey || e.altKey || e.ctrlKey || e.shiftKey || e.isComposing) {
      return
    }

    let history = this.props.history
    switch (key) {
      case "1":
        history.push(this.props.logUrl)
        e.preventDefault()
        break

      case "2":
        history.push(this.props.alertsUrl)
        e.preventDefault()
        break

      case "3":
        if (!this.props.facetsUrl) {
          return
        }
        history.push(this.props.facetsUrl)
        e.preventDefault()
        break
    }
  }

  render() {
    return <span></span>
  }
}

type PublicProps = {
  logUrl: string
  alertsUrl: string
  facetsUrl: string | null
}

export default function(props: PublicProps) {
  let history = useHistory()
  return <SecondaryNavKeyboardShortcuts {...props} history={history} />
}
