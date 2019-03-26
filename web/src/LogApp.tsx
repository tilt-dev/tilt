import React, { Component } from 'react';
import AppController from './AppController';
import LoadingScreen from './LoadingScreen';
import Ansi from "ansi-to-react";
import Helmet from 'react-helmet';
import Select from 'react-select';
import {withRouter, Link} from 'react-router-dom';
import {colorGreyDarkest, colorGreyDark} from './constants';
import './LogApp.scss';

let AnsiLine = React.memo(function(props: any) {
  return <div><Ansi linkify={false}>{props.line}</Ansi></div>
})

function titleText(name: string) {
  if (name) {
    return `Logs (${name})`
  }
  return 'All Logs'
}

var LogHeader = withRouter((props: any) => {
  let history = props.history
  let name = props.name
  let resourceNames = (props.resourceNames || []).filter((resName: string) => resName !== 'k8s_yaml')
  let allOption = {value: 'all', label: 'All'}
  let options = [allOption].concat(resourceNames.map((resName: string) => {
    return {value: resName, label: resName}
  }))

  let defaultValue = name ? {value: name, label: name} : allOption
  let styles = selectStyles()

  function onChange(inputValue: any, event: any) {
    let action = event.action
    let value = inputValue.value
    if (action !== 'select-option' || value === defaultValue.value || !value) {
      return
    }

    if (value === 'all') {
      history.push('/log')
    } else {
      history.push(`/r/${value}/log`)
    }
  }

  return (<header className="LogApp-header">
    <div className="LogApp-title"><Link to="/">Dashboard</Link>&nbsp;&gt;&nbsp;Logs: </div>
    <div className="LogApp-select">
      <Select isSearchable={true} defaultValue={defaultValue} options={options} styles={styles} onChange={onChange} />
    </div>
  </header>)
})

function selectStyles(): any {
  let control = (styles: Array<string>) => {
    return {...styles, backgroundColor: colorGreyDarkest, color: 'white'}
  }
  let option = (styles: any, {data, isFocused, isSelected}: {data: any, isFocused: boolean, isSelected: boolean}) => {
    let obj = {...styles}
    if (data.value !== 'all') {
      obj.paddingLeft = '1em'
    }
    obj.backgroundColor = colorGreyDarkest
    if (isFocused) {
      obj.backgroundColor = colorGreyDark
    }
    return obj
  }
  let singleValue = (styles: any) => {
    return {...styles, color: 'white'}
  }
  let menu = (styles: any) => {
    return {...styles, backgroundColor: colorGreyDarkest}
  }
  return {control, option, singleValue, menu}
}

class LogAppContents extends Component {
  _lastEl: any
  state: any
  _scrollTimeout: NodeJS.Timeout|null = null
  props: any

  constructor(props: any) {
    super(props)

    this.state = {
      autoscroll: true,
    }
    this._lastEl = null
    this.refreshAutoScroll = this.refreshAutoScroll.bind(this)
  }

  componentDidMount() {
    if (this._lastEl) {
      this._lastEl.scrollIntoView()
    }
    window.addEventListener('scroll', this.refreshAutoScroll, {passive: true})
  }

  componentDidUpdate() {
    if (!this.state.autoscroll) {
      return
    }
    if (this._lastEl) {
      this._lastEl.scrollIntoView()
    }
  }

  componentWillUnmount() {
    window.removeEventListener('scroll', this.refreshAutoScroll)
  }

  refreshAutoScroll() {
    if (this._scrollTimeout) {
      clearTimeout(this._scrollTimeout)
    }

    this._scrollTimeout = setTimeout(() => {
      let lastElInView = this._lastEl && (this._lastEl.getBoundingClientRect().top < window.innerHeight)

      // Always auto-scroll when we're recovering from a loading screen.
      let autoscroll = false
      if (!this.props.log || !this._lastEl) {
        autoscroll = true
      } else {
        autoscroll = lastElInView
      }

      this.setState({autoscroll})
    }, 250)
  }

  render() {
    let message = this.props.message
    let log = this.props.log
    let els = []
    if (!log) {
      els.push(<LoadingScreen key={"loading"} message={message} />)
    } else {
      let lines = log.split('\n')
      els = lines.map((line: string, i: number) => {
        return <AnsiLine key={'logLine' + i} line={line} />
      })
      els.push(
        <div key="logEnd" className="logEnd" ref={(el) => { this._lastEl = el }}>&#9608;</div>)
    }

    return (<React.Fragment>
      {els}
    </React.Fragment>)
  }
}

class LogApp extends Component {
  controller: AppController
  props: any
  state: any

  constructor(props: any) {
    super(props)

    this.controller = new AppController(`ws://${window.location.host}/ws/view`, this)
    this.state = {
      View: null,
      Message: '',
    }
  }

  componentDidMount() {
    this.controller.createNewSocket()
  }

  componentWillUnmount() {
    this.controller.dispose()
  }

  name() {
    return this.props.match.params.name
  }

  inferNewLog() {
    let state = this.state
    let view = state.View
    if (!view) {
      return {log: '', message: state.Message}
    }

    let name = this.name()
    let isGlobalLog = !name
    let resources = view.Resources || []
    if (isGlobalLog) {
      let log = (state.View && state.View.Log) || ''
      return {log: log, message: state.Message}
    }

    let resource = resources.find((res: any) => res.Name === name)
    if (!resource) {
      return {log: '', message: `Resource not found: ${name}`}
    }

    return {log: resource.CombinedLog || '', message: state.Message}
  }

  setAppState(state: any) {
    this.setState({View: state.View, Message: state.Message})
  }

  render() {
    let state = this.state
    let {log, message} = this.inferNewLog()
    let name = this.name()
    let title = titleText(name)
    let resources: Array<any> = (state.View && state.View.Resources) || []
    let resourceNames = resources.map((res) => res.Name)
      .filter((name: string) => name !== 'k8s_yaml')

    return (
      <div className="LogApp">
        <Helmet>
          <title>Tilt — {title}</title>
        </Helmet>
        <LogHeader name={name} resourceNames={resourceNames} />
        <LogAppContents log={log} message={message} />
      </div>
    );
  }
}

export default LogApp;
