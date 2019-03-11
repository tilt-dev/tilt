import React from 'react';
import ReactDOM from 'react-dom';
import App from './App';
import { mount } from 'enzyme';

it('renders without crashing', () => {
  const div = document.createElement('div');
  ReactDOM.render(<App />, div);
  ReactDOM.unmountComponentAtNode(div);
});

it('renders loading screen', async () => {
  const app = mount(<App />);
  expect(app.html()).toEqual(expect.stringContaining('Loading'))

  app.setState({Message: 'Disconnected'})
  expect(app.html()).toEqual(expect.stringContaining('Disconnected'))
});

it('renders resource', async () => {
  const app = mount(<App />);
  const ts = Date.now().toLocaleString()
  const resource = {
    Name: "vigoda",
    DirectoriesWatched: ["foo", "bar"],
    LastDeployTime: ts,
    BuildHistory: [{
      Edits: ["main.go", "cli.go"],
      Error: "the build failed!",
      FinishTime: ts,
      StartTime: ts,
    }],
    PendingBuildEdits: ["main.go", "cli.go", "vigoda.go"],
    PendingBuildSince: ts,
    CurrentBuild: {
      Edits: ["main.go"],
      StartTime: ts,
    },
    PodName: "vigoda-pod",
    PodCreationTime: ts,
    PodStatus: "Running",
    PodRestarts: 1,
    Endpoints: ["1.2.3.4:8080"],
    PodLog: "1\n2\n3\n4\nabe vigoda is now dead\n5\n6\n7\n8\n",
  }
  app.setState({View: {Resources: [resource]}})
  expect(app.find('.name')).toHaveLength(1);
  expect(app.find('.name').html()).toEqual(expect.stringContaining('vigoda'))
});
