import React, { Component } from 'react';
import ResourceList from './ResourceList';
import Preview from './Preview';
import Status from './Status';
import AppController from './AppController';
import LoadingScreen from './LoadingScreen';
import './App.css';

class App extends Component {
  constructor(props) {
    super(props)

    this.controller = new AppController(`ws://${window.location.host}/ws/view`, this)
    this.state = {
      Message: '',
      View: {Resources: []},
      ShowPreview: false,
    }
    this.togglePreview = this.togglePreview.bind(this);
  }

  togglePreview() {
    this.setState({ShowPreview: !this.state.ShowPreview})
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
          el = <Preview key="Preview" resources={view.Resources} />
      } else {
          el = <ResourceList key="ResourceList" resources={view.Resources} />
      }
    }

    return (
      <React.Fragment>
        {el}
        <Status key="Status" togglePreview={this.togglePreview} resources={view.Resources} />
      </React.Fragment>
    );
  }
}

export default App;
