import React from 'react';
import ReactDOM from 'react-dom';
import './index.css';
import App from './App';

// Assume that the HUD was started with --port=8001
let url = 'http://localhost:8001/api/view'
let renderAsync = new Promise((resolve, reject) => {
  let app = (<App />)
  let root = document.getElementById('root')
  ReactDOM.render(app, root, function() {
    resolve(this)
  })
})

let fetchAsync = fetch(url)
    .then((body) => body.text())
    .then(JSON.parse)

Promise.all([
  renderAsync,
  fetchAsync
]).then((values) => {
  let [component, data] = values
  component.setState(data)
})
