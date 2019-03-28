import React from 'react'
import ReactDOM from 'react-dom'
import renderer from 'react-test-renderer';
import Statusbar, {StatusItem} from './Statusbar'

describe('StatusBar', () => {
  it('renders without crashing', () => {
    const tree = renderer
      .create(<Statusbar items={[]} toggleSidebar={null} />)
      .toJSON()

    expect(tree).toMatchSnapshot()
  })
})

describe('StatusItem', () => {
  it('can be constructed with no build history', () => {
    let si = new StatusItem({})
  })
})
