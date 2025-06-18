import React, {
  PropsWithChildren,
  useContext,
  useEffect,
  useState,
} from "react"
import { usePersistentState } from "./BrowserStorage"

export type StarredResourcesContext = {
  starredResources: string[]
  starResource: (name: string) => void
  unstarResource: (name: string) => void
  toggleStarResource: (name: string) => void
}

const starredResourceContext = React.createContext<StarredResourcesContext>({
  starredResources: [],
  starResource: (s) => {},
  unstarResource: (s) => {},
  toggleStarResource: (s) => {},
})

export function useStarredResources(): StarredResourcesContext {
  return useContext(starredResourceContext)
}

export function StarredResourceMemoryProvider(
  props: PropsWithChildren<{ initialValueForTesting?: string[] }>
) {
  const [starredResources, setStarredResources] = useState<Array<string>>(
    props.initialValueForTesting || []
  )

  function starResource(name: string) {
    setStarredResources((prevState) => {
      return prevState.includes(name) ? prevState : [...prevState, name]
    })
  }

  function unstarResource(name: string) {
    setStarredResources((prevState) => {
      return prevState.filter((s) => s !== name)
    })
  }

  function toggleStarResource(name: string) {
    if (starredResources.includes(name)) {
      unstarResource(name)
    } else {
      starResource(name)
    }
  }

  return (
    <starredResourceContext.Provider
      value={{
        starredResources: starredResources,
        starResource: starResource,
        unstarResource: unstarResource,
        toggleStarResource: toggleStarResource,
      }}
    >
      {props.children}
    </starredResourceContext.Provider>
  )
}

export function StarredResourcesContextProvider(
  props: PropsWithChildren<{ initialValueForTesting?: string[] }>
) {
  // we renamed pins to stars but kept the local storage name "pinned-resources"
  // so that user's pinned resources show up as starred
  let [starredResources, setStarredResources] = usePersistentState<string[]>(
    "pinned-resources",
    props.initialValueForTesting ?? []
  )

  useEffect(() => {}, [])

  function starResource(name: string) {
    setStarredResources((prevState) => {
      const ret = prevState.includes(name) ? prevState : [...prevState, name]
      return ret
    })
  }

  function unstarResource(name: string) {
    setStarredResources((prevState) => {
      const ret = prevState.filter((n) => n !== name)
      return ret
    })
  }

  function toggleStarResource(name: string) {
    if (starredResources.includes(name)) {
      unstarResource(name)
    } else {
      starResource(name)
    }
  }

  return (
    <starredResourceContext.Provider
      value={{
        starredResources: starredResources,
        starResource: starResource,
        unstarResource: unstarResource,
        toggleStarResource: toggleStarResource,
      }}
    >
      {props.children}
    </starredResourceContext.Provider>
  )
}
