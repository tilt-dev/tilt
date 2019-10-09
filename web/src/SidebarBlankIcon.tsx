import React, { PureComponent } from "react"
import { ReactComponent as ManualSvg } from "./assets/svg/indicator-manual.svg"

type SidebarBlankIconProps = {
}


export default class SidebarBlankIcon extends PureComponent<SidebarBlankIconProps> {
    render() {
        return (<ManualSvg />)
    }
}


