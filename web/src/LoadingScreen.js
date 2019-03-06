import React from 'react';
import './LoadingScreen.css';

function LoadingScreen(props) {
  let message = props.message || 'Loading…'
  return (
    <header className="LoadingScreen">
      {message}
    </header>
  )
}

export default LoadingScreen;
