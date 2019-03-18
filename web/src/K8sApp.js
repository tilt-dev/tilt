import React, { Component } from 'react';
import AppController from './AppController';
import { Link, withRouter } from 'react-router-dom';
import {colorGreyDarkest, colorGreyDark} from './constants';
import Select from 'react-select';
import './K8sApp.scss';

var K8sHeader = withRouter((props) => {
  let history = props.history
  let name = props.name
  let options = props.items.map((item) => {
    return {value: item.name, label: item.name}
  })

  let defaultValue = {value: name, label: name}
  let styles = selectStyles()

  function onChange(inputValue, event) {
    let action = event.action
    let value = inputValue.value
    if (action !== 'select-option' || value === defaultValue.value || !value) {
      return
    }
    history.push(`/r/${value}/k8s`)
  }

  return (<header className="K8sApp-header">
    <div className="K8sApp-title"><Link to="/">Dashboard</Link>&nbsp;&gt;&nbsp;K8s Resources: </div>
    <div className="K8sApp-select">
      <Select isSearchable={true} defaultValue={defaultValue} options={options} styles={styles} onChange={onChange} />
    </div>
  </header>)
})

function selectStyles() {
  let control = (styles) => {
    return {...styles, backgroundColor: colorGreyDarkest, color: 'white'}
  }
  let option = (styles, {data, isFocused, isSelected}) => {
    let obj = {...styles}
    obj.backgroundColor = colorGreyDarkest
    if (isFocused) {
      obj.backgroundColor = colorGreyDark
    }
    return obj
  }
  let singleValue = (styles) => {
    return {...styles, color: 'white'}
  }
  let menu = (styles) => {
    return {...styles, backgroundColor: colorGreyDarkest}
  }
  return {control, option, singleValue, menu}
}

let K8sContentPane = React.memo((props) => {
  return (<div className="K8sContentPane">
    <pre>{props.yaml}</pre>
  </div>)
})

// Displays the deployed YAML for Kubernetes resources.
class K8sApp extends Component {
  constructor(props) {
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

  setAppState(state) {
    this.setState({View: state.View, Message: state.Message})
  }

  render() {
    let resources = (this.state.View && this.state.View.Resources) || []
    let name = this.props.match.params.name
    let selectedYaml = ''
    let items = resources.map((res) => {
      let resInfo = res.ResourceInfo || {}
      let yaml = resInfo.YAML || ''
      let isSelected = name === res.Name
      if (isSelected) {
        selectedYaml = yaml
      }
      return {
        name: res.Name,
        isK8s: !!yaml,
        isSelected,
      }
    }).filter((item) => item.isK8s)

    return (
      <div className="K8sApp">
        <K8sHeader name={name} items={items} />
        <K8sContentPane name={name} yaml={selectedYaml} />
      </div>
    );
  }
}

export default K8sApp;
