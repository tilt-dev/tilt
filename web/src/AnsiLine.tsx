import React from "react"
import Ansi from "ansi-to-react"
import "./AnsiLine.scss"

type AnsiLineProps = {
  line: string
  className?: string
}

let AnsiLine = React.memo(function(props: AnsiLineProps) {
  return (
    <React.Fragment>
      <Ansi linkify={false} useClasses={true} className={props.className}>
        {props.line + "\n"}
      </Ansi>
    </React.Fragment>
  )
})

export default AnsiLine
