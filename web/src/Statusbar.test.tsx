import React from 'react'
import ReactDOM from 'react-dom'
import renderer from 'react-test-renderer';
import Statusbar from './Statusbar'

it('renders without crashing if theres no last build', () => {
  const tree = renderer
    .create(<Statusbar items={[]} toggleSidebar={null} />)
    .toJSON()

  expect(tree).toMatchSnapshot()
})
