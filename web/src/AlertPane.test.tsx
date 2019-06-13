import React from "react"
import AlertPane, { AlertResource } from "./AlertPane"
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
    .create(<AlertPane resources={resources.map(r => new AlertResource(r))} />)
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
    .create(<AlertPane resources={resources.map(r => new AlertResource(r))} />)
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
    .create(<AlertPane resources={resources.map(r => new AlertResource(r))} />)
    .toJSON()

  expect(tree).toMatchSnapshot()
})

it("renders one container start error", () => {
  const ts = "1,555,970,585,039"
  let resources = [
    {
      Name: "foo",
      CrashLog: "Eeeeek there is a problem",
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
      },
    },
  ]

  const tree = renderer
    .create(<AlertPane resources={resources.map(r => new AlertResource(r))} />)
    .toJSON()
  expect(tree).toMatchSnapshot()

  // the podStatus will flap between "Error" and "CrashLoopBackoff"
  resources = [
    {
      Name: "foo",
      CrashLog: "Eeeeek there is a problem",
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
      },
    },
  ]

  const newTree = renderer
    .create(<AlertPane resources={resources.map(r => new AlertResource(r))} />)
    .toJSON()
  expect(newTree).toMatchSnapshot()
})

it("shows that a container has restarted", () => {
  const ts = "1,555,970,585,039"
  const resources = [
    {
      Name: "foo",
      CrashLog: "Eeeeek the container crashed",
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
      },
    },
  ]

  const tree = renderer
    .create(<AlertPane resources={resources.map(r => new AlertResource(r))} />)
    .toJSON()
  expect(tree).toMatchSnapshot()
})

it("shows that a crash rebuild has occurred", () => {
  const ts = "1,555,970,585,039"
  const resources = [
    {
      Name: "foo",
      CrashLog: "Eeeeek the container crashed",
      BuildHistory: [
        {
          Log: "laa dee daa I'm not an error\nseriously",
          FinishTime: ts,
          Error: null,
          IsCrashRebuild: true,
        },
      ],
      ResourceInfo: {
        PodCreationTime: ts,
        PodStatus: "ok",
      },
    },
  ]

  const tree = renderer
    .create(<AlertPane resources={resources.map(r => new AlertResource(r))} />)
    .toJSON()
  expect(tree).toMatchSnapshot()
})

it("renders multiple lines of a crash log", () => {
  const ts = "1,555,970,585,039"
  const resources = [
    {
      Name: "foo",
      CrashLog: "Eeeeek the container crashed\nno but really it crashed",
      BuildHistory: [
        {
          Log: "laa dee daa I'm not an error\nseriously",
          FinishTime: ts,
          Error: null,
          IsCrashRebuild: true,
        },
      ],
      ResourceInfo: {
        PodCreationTime: ts,
        PodStatus: "ok",
      },
    },
  ]

  const tree = renderer
    .create(<AlertPane resources={resources.map(r => new AlertResource(r))} />)
    .toJSON()
  expect(tree).toMatchSnapshot()
})

it("renders warnings", () => {
  const ts = "1,555,970,585,039"
  const resources = [
    {
      Name: "foo",
      CrashLog: "Eeeeek the container crashed",
      BuildHistory: [
        {
          Log: "laa dee daa I'm not an error\nseriously",
          FinishTime: ts,
          Error: null,
          IsCrashRebuild: true,
          Warnings: ["Hi I'm a warning"],
        },
      ],
      ResourceInfo: {
        PodCreationTime: ts,
        PodStatus: "ok",
      },
    },
  ]

  const tree = renderer
    .create(<AlertPane resources={resources.map(r => new AlertResource(r))} />)
    .toJSON()
  expect(tree).toMatchSnapshot()
})
