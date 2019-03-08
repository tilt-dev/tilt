import React from 'react';
import ReactDOM from 'react-dom';
import './index.css';
import App from './App';
import LogApp from './LogApp';
import Preview from './Preview';
import LoadingScreen from './LoadingScreen';
import { BrowserRouter as Router, Route, Switch } from 'react-router-dom';

let NoMatch = ({location}) => {
  let message = (<div>No match for <code>{location.pathname}</code></div>)
  return <LoadingScreen message={message} />
};

let Main = () => {
  return (<Router>
    <main>
      <Switch>
        <Route exact path="/" component={App} />
        <Route exact path="/log" component={LogApp} />
        <Route exact path="/preview" component={Preview} />
        <Route component={NoMatch} />
      </Switch>
    </main>
  </Router>)
}

let app = (<Main />)
let root = document.getElementById('root')
ReactDOM.render(app, root)
