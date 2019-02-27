import React from 'react';
import ReactDOM from 'react-dom';
import './index.css';
import App from './App';
import { BrowserRouter as Router, Route } from 'react-router-dom';

let Main = () => {
  return (<Router>
    <div>
      <Route exact path="/" component={App} />
    </div>
  </Router>)
}

let app = (<Main />)
let root = document.getElementById('root')
ReactDOM.render(app, root)
