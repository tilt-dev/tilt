import { makeStyles } from "@material-ui/core/styles"
import { Font, FontSize } from "./style-helpers"

// Classes for a MUI MenuItem in Tilt's dropdown menus.
// Usage: <MenuItem classes={useMenuItemStyles()}>
export const useMenuItemStyles = makeStyles((theme: any) => ({
  root: {
    fontFamily: Font.sansSerif,
    fontSize: FontSize.smallest,
  },
}))
