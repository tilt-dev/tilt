import React from "react"
import AlertPane from "./AlertPane"
import renderer from "react-test-renderer"
import {oneResourceUnrecognizedError} from "./testdata.test"
import {Resource, ResourceInfo, TriggerMode} from "./types";
import {Alert,  PodRestartErrorType, PodStatusErrorType,ResourceCrashRebuildErrorType, BuildFailedErrorType, WarningErrorType} from "./alerts";




beforeEach(() => {
  Date.now = jest.fn(() => 1482363367071)
})

it("renders no errors", () => {
   let resources: Array<Partial<Resource>> = [
    {
      Name: "foo",
      Alerts: [],
    },
  ]

  const tree = renderer
    .create(<AlertPane resources={resources as Array<Resource>} />)
    .toJSON()

  expect(tree).toMatchSnapshot()
})

it("renders one build error", () => {
  const ts = "1,555,970,585,039"
  let resources:Array<Partial<Resource>> = [
    {
      Name: "foo",
      Alerts: [{alertType:BuildFailedErrorType, msg: "laa dee daa I'm an error\nfor real I am", titleMsg: "", timestamp: ts}]
    },
  ]

  const tree = renderer
    .create(<AlertPane resources={resources as Array<Resource>} />)
    .toJSON()

  expect(tree).toMatchSnapshot()
})

it("renders the last build with an error", () => {
  const ts = "1,555,970,585,039"
  let resources: Array<Partial<Resource>> = [
    {
      Name: "foo",
      Alerts: [
          {alertType:PodRestartErrorType, msg: "laa dee daa I'm an error\nfor real I am", titleMsg: "", timestamp: ts},
          {alertType:PodRestartErrorType, msg: "\"laa dee daa I'm an error\nI'm serious", titleMsg: "", timestamp: ts}
      ]
    },
  ]

  const tree = renderer
    .create(<AlertPane resources={resources as Array<Resource>} />)
    .toJSON()

  expect(tree).toMatchSnapshot()
})

it("renders one container start error", () => {
  const ts = "1,555,970,585,039"
  let resource: Partial<Resource> = {
    Name: "foo",

  }
  let resources = [
    {
      Name: "foo",
      Alerts: [
        {alertType:PodRestartErrorType, msg: "", titleMsg: "", timestamp: ts},
      ]
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
    .create(<AlertPane resources={resources as Array<Resource>} />)
    .toJSON()
  expect(tree).toMatchSnapshot()

  // the podStatus will flap between "Error" and "CrashLoopBackOff"
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
    .create(<AlertPane resources={resources as Array<Resource>} />)
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
    .create(<AlertPane resources={resources as Array<Resource>} />)
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
    .create(<AlertPane resources={resources as Array<Resource>} />)
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
    .create(<AlertPane resources={resources as Array<Resource>} />)
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
    .create(<AlertPane resources={resources as Array<Resource>} />)
    .toJSON()
  expect(tree).toMatchSnapshot()
})

it("renders one container unrecognized error", () => {
  const ts = "1,555,970,585,039"
  let resources = [oneResourceUnrecognizedError()]
  const tree = renderer
    .create(<AlertPane resources={resources} />)
    .toJSON()
  expect(tree).toMatchSnapshot()
})

function fillResourceFields() : Resource{
  return {
    Name: "",
    CombinedLog: "",
    BuildHistory:[],
    CrashLog: "",
    CurrentBuild: 0,
    DirectoriesWatched: [],
    Endpoints: [],
    PodID: "",
    IsTiltfile: false,
    LastDeployTime: "",
    PathsWatched: [],
    PendingBuildEdits:[],
    PendingBuildReason: 0,
    PendingBuildSince: "",
    ResourceInfo: {
      PodName: "",
      PodCreationTime: "",
      PodUpdateStartTime: "",
      PodStatus: "",
      PodStatusMessage: "",
      PodRestarts: 0,
      PodLog: "",
      YAML: "",
      Endpoints: [],
    },
    RuntimeStatus: "",
    TriggerMode: TriggerMode.TriggerModeAuto,
    HasPendingChanges: true,
    Alerts: [],
  }
}