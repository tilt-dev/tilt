import { Typography, TypographyTypeMap } from "@material-ui/core"
import {
  DefaultComponentProps,
  OverrideProps,
} from "@material-ui/core/OverridableComponent"
import React, { ElementType, PropsWithChildren } from "react"

/**
 * A lightweight wrapper for content that should only be available
 * to assistive technology, using Material UI's Typography component
 * with `srOnly` class. Screen-reader-only classes are a common pattern
 * that allows useful content to be present in the DOM (and therefore
 * available to screen-readers), but not visible to sighted users.
 * https://material-ui.com/api/typography/
 */

// Note: types are copy-pasta'd and adapted from Typography.d.ts
type SrOnlyProps<C extends ElementType> =
  | ({ component: C } & OverrideProps<TypographyTypeMap, C> &
      DefaultComponentProps<TypographyTypeMap>)
  | DefaultComponentProps<TypographyTypeMap>

export default function SrOnly<C extends ElementType = "span">(
  props: PropsWithChildren<SrOnlyProps<C>>
) {
  return (
    <Typography {...props} variant="srOnly">
      {props.children}
    </Typography>
  )
}
