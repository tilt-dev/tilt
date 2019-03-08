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
    }
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
      el = <React.Fragment>
        <ResourceList key="ResourceList" resources={view.Resources} />
        <Preview key="Preview" resources={view.Resources} />
        <Status key="Status" resources={view.Resources} />
      </React.Fragment>
    }

    return (
      <React.Fragment>
        {el}
      </React.Fragment>
    );
  }
}

export default App;
