import React, { Component } from 'react';
import AppController from './AppController';
import LoadingScreen from './LoadingScreen';
import Ansi from "ansi-to-react";
import './LogApp.css';

let AnsiLine = React.memo(function(props) {
  return <div><Ansi>{props.line}</Ansi></div>
})

class LogApp extends Component {
  constructor(props) {
    super(props)

    this.controller = new AppController(`ws://${window.location.host}/ws/view`, this)
    this.state = {
      log: '',
      message: '',
      autoscroll: true,
    }
    this._lastEl = null
  }

  componentDidMount() {
    this.controller.createNewSocket()
    this._lastEl.scrollIntoView()
  }

  componentDidUpdate() {
    if (!this.state.autoscroll) {
      return
    }
    this._lastEl.scrollIntoView()
  }

  componentWillUnmount() {
    this.controller.dispose()
  }

  inferNewLog(state) {
    let view = state.View
    if (!view) {
      return {message: state.Message}
    }

    let name = this.props.match.params.name
    let isGlobalLog = !name
    if (isGlobalLog) {
      let log = (state.View && state.View.Log) || ''
      return {log: log, message: state.Message}
    }

    let resources = view.Resources || []
    let resource = resources.find((res) => res.Name === name)
    if (!resource) {
      return {message: `Resource not found: ${name}`}
    }

    return {log: resource.CombinedLog, message: state.Message}
  }

  setAppState(state) {
    let {log, message} = this.inferNewLog(state)
    let lastElInView = this._lastEl && (this._lastEl.getBoundingClientRect().top < window.innerHeight)
    this.setState((prevState) => {
      // Always auto-scroll when we're recovering from a loading screen.
      let shouldAutoScroll = false
      if (!prevState.log || !this._lastEl) {
        shouldAutoScroll = true
      } else {
        shouldAutoScroll = lastElInView
      }
      return {
        autoscroll: shouldAutoScroll,
        log: log || '',
        message: message || '',
      }
    })
  }

  render() {
    let els = []
    let log = this.state.log
    let message = this.state.message
    if (!log) {
      els.push(<LoadingScreen key={"loading"} message={message} />)
    } else {
      let lines = log.split('\n')
      els = lines.map((line, i) => {
        return <AnsiLine key={i} line={line} />
      })
    }

    return (
      <div className="LogApp">
        {els}
        <div className="logEnd" ref={(el) => { this._lastEl = el }}>&#9608;</div>
      </div>
    );
  }
}

export default LogApp;
