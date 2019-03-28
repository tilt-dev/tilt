import React from 'react'
import ReactDOM from 'react-dom'
import HUD from './HUD'
import { mount } from 'enzyme'

const emptyHUD = <HUD history={null} location={null} match={null} />

it('renders without crashing', () => {
  const div = document.createElement('div')
  ReactDOM.render(emptyHUD, div)
  ReactDOM.unmountComponentAtNode(div)
});

it('renders loading screen', async () => {
  const hud = mount(emptyHUD)
  expect(hud.html()).toEqual(expect.stringContaining('Loading'))

  hud.setState({Message: 'Disconnected'})
  expect(hud.html()).toEqual(expect.stringContaining('Disconnected'))
});

it('renders resource', async () => {
  const hud = mount(emptyHUD);
  hud.setState({View: oneResourceView()})
  expect(hud.html())
  expect(hud.find('.Statusbar')).toHaveLength(1)
  expect(hud.find('.Sidebar')).toHaveLength(1)
});

it('opens sidebar on click', async () => {
  const hud = mount(emptyHUD);
  hud.setState({View: oneResourceView()})

  let sidebar = hud.find('.Sidebar')
  expect(sidebar).toHaveLength(1)
  expect(sidebar.hasClass('is-open')).toBe(false)

  let button = hud.find('button.Statusbar-panel--up')
  expect(button).toHaveLength(1)
  button.simulate('click')

  sidebar = hud.find('.Sidebar')
  expect(sidebar).toHaveLength(1)
  expect(sidebar.hasClass('is-open')).toBe(true)
});

function oneResourceView() {
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
  return {Resources: [resource]}
}
