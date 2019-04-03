import { RouteComponentProps } from "react-router-dom"
import { UnregisterCallback, Href } from "history"

// NOTE(dmiller) 4-02-19 this function is currently unused but I'm going to keep it around.
// I have a feeling that it will come in handy later.
function getMockRouterProps<P>(data: P) {
  var location = {
    hash: "",
    key: "",
    pathname: "",
    search: "",
    state: {},
  }

  var props: RouteComponentProps<P> = {
    match: {
      isExact: true,
      params: data,
      path: "",
      url: "",
    },
    location: location,
    history: {
      length: 2,
      action: "POP",
      location: location,
      push: () => {},
      replace: () => {},
      go: num => {},
      goBack: () => {},
      goForward: () => {},
      block: t => {
        var temp: UnregisterCallback = () => {}
        return temp
      },
      createHref: t => {
        var temp: Href = ""
        return temp
      },
      listen: t => {
        var temp: UnregisterCallback = () => {}
        return temp
      },
    },
    staticContext: {},
  }

  return props
}

function oneResource(): any {
  const ts = Date.now().toLocaleString()
  const resource = {
    Name: "vigoda",
    DirectoriesWatched: ["foo", "bar"],
    LastDeployTime: ts,
    BuildHistory: [
      {
        Edits: ["main.go", "cli.go"],
        Error: "the build failed!",
        FinishTime: ts,
        StartTime: ts,
      },
    ],
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
    RuntimeStatus: "ok",
  }
  return resource
}

function oneResourceView(): any {
  return { Resources: [oneResource()] }
}

function twoResourceView(): any {
  const ts = Date.now().toLocaleString()
  const vigoda = oneResource()

  const snack = {
    Name: "snack",
    DirectoriesWatched: ["foo", "bar"],
    LastDeployTime: ts,
    BuildHistory: [
      {
        Edits: ["main.go", "cli.go"],
        Error: "the build failed!",
        FinishTime: ts,
        StartTime: ts,
      },
    ],
    PendingBuildEdits: ["main.go", "cli.go", "snack.go"],
    PendingBuildSince: ts,
    CurrentBuild: {
      Edits: ["main.go"],
      StartTime: ts,
    },
    PodName: "snack-pod",
    PodCreationTime: ts,
    PodStatus: "Running",
    PodRestarts: 1,
    Endpoints: ["1.2.3.4:8080"],
    PodLog: "1\n2\n3\n4\nsnacks are great\n5\n6\n7\n8\n",
    RuntimeStatus: "ok",
  }
  return { Resources: [vigoda, snack] }
}

it("loads ok", () => {})
export { oneResource, oneResourceView, twoResourceView, getMockRouterProps }
