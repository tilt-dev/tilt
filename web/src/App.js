import React, { Component } from 'react';
import ResourceList from './ResourceList';
import './App.css';

class App extends Component {
  constructor(props) {
    super(props)
    this.state = {
      Message: '',
      View: {Resources: []},
    }
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

function LoadingScreen(props) {
  let message = props.message || 'Loading…'
  return (
    <header className="LoadingScreen">
      {message}
    </header>
  )
}

export default App;
