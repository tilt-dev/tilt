import React from "react"
import ErrorPane, { ErrorResource } from "./ErrorPane"
import renderer from "react-test-renderer"

beforeEach(() => {
  Date.now = jest.fn(() => 1482363367071)
})

it("renders no errors", () => {
  let resources = [
    {
      Name: "foo",
      BuildHistory: [],
      ResourceInfo: {},
    },
  ]

  const tree = renderer
    .create(<ErrorPane resources={resources.map(r => new ErrorResource(r))} />)
    .toJSON()

  expect(tree).toMatchSnapshot()
})

it("renders one build error", () => {
  const ts = "1,555,970,585,039"
  let resources = [
    {
      Name: "foo",
      BuildHistory: [
        {
          Log: "laa dee daa I'm an error\nfor real I am",
          FinishTime: ts,
          Error: {},
        },
      ],
      ResourceInfo: {},
    },
  ]

  const tree = renderer
    .create(<ErrorPane resources={resources.map(r => new ErrorResource(r))} />)
    .toJSON()

  expect(tree).toMatchSnapshot()
})

it("renders the last build with an error", () => {
  const ts = "1,555,970,585,039"
  let resources = [
    {
      Name: "foo",
      BuildHistory: [
        {
          Log: "laa dee daa I'm another error\nBetter watch out",
          FinishTime: ts,
          Error: {},
        },
        {
          Log: "laa dee daa I'm an error\nI'm serious",
          FinishTime: ts,
          Error: {},
        },
      ],
      ResourceInfo: {},
    },
  ]

  const tree = renderer
    .create(<ErrorPane resources={resources.map(r => new ErrorResource(r))} />)
    .toJSON()

  expect(tree).toMatchSnapshot()
})

it("renders one container start error", () => {
  const ts = "1,555,970,585,039"
  let resources = [
    {
      Name: "foo",
      BuildHistory: [
        {
          Log: "laa dee daa I'm an error\nI'm serious",
          FinishTime: ts,
          Error: null,
        },
      ],
      ResourceInfo: {
        PodCreationTime: ts,
        PodStatus: "Error",
        PodRestarts: 2,
        PodLog: "Eeeeek there is a problem",
      },
    },
  ]

  const tree = renderer
    .create(<ErrorPane resources={resources.map(r => new ErrorResource(r))} />)
    .toJSON()
  expect(tree).toMatchSnapshot()

  // the podStatus will flap between "Error" and "CrashLoopBackoff"
  resources = [
    {
      Name: "foo",
      BuildHistory: [
        {
          Log: "laa dee daa I'm not an error\nI'm serious",
          FinishTime: ts,
          Error: null,
        },
      ],
      ResourceInfo: {
        PodCreationTime: ts,
        PodStatus: "CrashLoopBackOff",
        PodRestarts: 3,
        PodLog: "Eeeeek there is a problem",
      },
    },
  ]

  const newTree = renderer
    .create(<ErrorPane resources={resources.map(r => new ErrorResource(r))} />)
    .toJSON()
  expect(newTree).toMatchSnapshot()
})

it("shows that a container has restarted", () => {
  const ts = "1,555,970,585,039"
  const resources = [
    {
      Name: "foo",
      BuildHistory: [
        {
          Log: "laa dee daa I'm not an error\nseriously",
          FinishTime: ts,
          Error: null,
        },
      ],
      ResourceInfo: {
        PodCreationTime: ts,
        PodStatus: "ok",
        PodRestarts: 1,
        PodLog: "Eeeeek the container crashed",
      },
    },
  ]

  const tree = renderer
    .create(<ErrorPane resources={resources.map(r => new ErrorResource(r))} />)
    .toJSON()
  expect(tree).toMatchSnapshot()
})
