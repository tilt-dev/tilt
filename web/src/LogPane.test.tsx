import React from 'react'
import ReactDOM from 'react-dom'
import LogPane from './LogPane'
import renderer from 'react-test-renderer';

it('renders without crashing', () => {
  let div = document.createElement('div')
  Element.prototype.scrollIntoView = jest.fn()
  ReactDOM.render(<LogPane log="hello\nworld\nfoo" message="world" />, div)
  ReactDOM.unmountComponentAtNode(div)
})

it('renders logs', () => {
  const log = "hello\nworld\nfoo\nbar"
  const tree = renderer
    .create(<LogPane log={log} />)
    .toJSON()

  expect(tree).toMatchSnapshot()
})
