import React, { Component } from 'react';
import AppController from './AppController';
import LoadingScreen from './LoadingScreen';
import './LogApp.css';

class LogApp extends Component {
  constructor(props) {
    super(props)

    this.controller = new AppController(`ws://${window.location.host}/ws/view`, this)
    this.state = {
      Log: '',
      Message: '',
    }
  }

  componentDidMount() {
    this.controller.createNewSocket()
  }

  componentWillUnmount() {
    this.controller.dispose()
  }

  setAppState(state) {
    let log = (state.View && state.View.Log) || ''
    this.setState({
      Log: log,
      Message: state.Message
    })
  }

  render() {
    let els = []
    let log = this.state.Log
    let message = this.state.Message
    if (!log) {
      els.push(<LoadingScreen message={message} />)
    } else {
      let lines = log.split('\n')
      els = lines.map((line) => {
        // TODO(nick): Add a LogLine component that properly handles
        // ANSI color sequences.
        return <div>{line}</div>
      })
    }

    return (
      <div className="LogApp">
        {els}
      </div>
    );
  }
}

export default LogApp;
