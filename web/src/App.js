import React, { Component } from 'react';
import ResourceList from './ResourceList';
import './App.css';

class App extends Component {
  constructor(props) {
    super(props)
    this.state = {Resources: []}
  }

  render() {
    let el = null
    if (!this.state.Resources || !this.state.Resources.length) {
      el = <LoadingScreen />
    } else {
      el = <ResourceList resources={this.state.Resources} />
    }

    return (
      <div className="App">
        {el}
      </div>
    );
  }
}

function LoadingScreen() {
  return (
    <header className="LoadingScreen">
      Loading...
    </header>
  )
}

export default App;
