import React from 'react';
import LoadingScreen from './LoadingScreen';

let NoMatch = ({location}) => {
  let message = (<div>No match for <code>{location.pathname}</code></div>)
  return <LoadingScreen message={message} />
};

export default NoMatch;
