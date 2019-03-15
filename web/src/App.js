import React, { Component } from 'react';
import ResourceList from './ResourceList';
import Status from './Status';
import AppController from './AppController';
import LoadingScreen from './LoadingScreen';
import './App.scss';

class App extends Component {
  constructor(props) {
    super(props)

    this.controller = new AppController(`ws://${window.location.host}/ws/view`, this)
    this.state = {
      Message: '',
      View: {Resources: []},
      ShowPreview: false,
      PreviewUrl: '',
    }
    this.openPreview = this.openPreview.bind(this);
    this.closePreview = this.closePreview.bind(this);
  }

  openPreview(endpoint) {
    this.setState({
      ShowPreview: true,
      PreviewUrl: endpoint,
    })
  }

  closePreview() {
    this.setState({ShowPreview: false})
  }

  componentDidMount() {
    this.controller.createNewSocket()
  }

  componentWillUnmount() {
    this.controller.dispose()
  }

  setAppState(state) {
    this.setState(state)
  }

  render() {
    let el = null
    let view = this.state.View
    let message = this.state.Message
    if (!view || !view.Resources || !view.Resources.length) {
      el = <LoadingScreen message={message} />
    } else {
      if (this.state.ShowPreview === true) {
        el = <iframe className="preview" title="preview" src={this.state.PreviewUrl}></iframe>
      } else {
        el = <ResourceList key="ResourceList" resources={view.Resources} openPreview={this.openPreview} />
      }
    }

    return (
      <React.Fragment>
        {el}
        <Status key="Status" closePreview={this.closePreview} showPreview={this.state.ShowPreview} resources={view.Resources} />
      </React.Fragment>
    );
  }
}

export default App;
