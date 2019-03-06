import React, { Component } from 'react';
import ResourceList from './ResourceList';
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
      el = <ResourceList resources={view.Resources} />
    }

    return (
      <div className="App">
        {el}
      </div>
    );
  }
}

export default App;
