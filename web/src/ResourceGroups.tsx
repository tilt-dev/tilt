import styled from "styled-components"
import { ReactComponent as CaretSvg } from "./assets/svg/caret.svg"
import { Color, SizeUnit } from "./style-helpers"

/**
 * This file contains style-reset versions of the Material UI Accordion
 * component (which ships with a bunch of opinionated, intricate styles
 * that don't align with our design implementation and can be difficult
 * to reset).
 *
 * This Accordion component is used for resource groups that are grouped
 * by label. This feature is composed of three components, structured like:
 *    <Accordion>
 *      <AccordionSummary> {content} </AccordionSummary>
 *      <AccordionDetails> {content} </AccordionDetails>
 *    </Accordion>
 *
 *    where Accordion is the parent wrapper component.
 *          AccordionSummary contains the clickable button that expands and
 *            collapses the content in Accordion details; the content in the
 *            Summary is always visible.
 *          AccordionDetails contains the content that is expanded and
 *            collapsed by clicking on the Summary.
 *
 * Note: the Accordion resource groups components are only used in the table
 * and detail views. If they need to be used in more locations, these components
 * and styles should be refactored to be more reusable.
 */

export const AccordionStyleResetMixin = `
  &.MuiPaper-root {
    background-color: unset;
  }

  &.MuiPaper-elevation1 {
    box-shadow: unset;
  }

  &.MuiAccordion-root,
  &.MuiAccordion-root.Mui-expanded {
    margin: unset;
    position: unset; /* Removes a mysterious top border only visible on collapse */
  }
`

export const AccordionSummaryStyleResetMixin = `
  &.MuiAccordionSummary-root,
  &.MuiAccordionSummary-root.Mui-expanded {
    min-height: unset;
    padding: unset;
  }

  .MuiAccordionSummary-content {
    margin: 0;

    &.Mui-expanded {
      margin: 0;
    }
  }
`

export const AccordionDetailsStyleResetMixin = `
  &.MuiAccordionDetails-root {
    display: unset;
    padding: unset;
  }
`

// Helper (non-Material UI) components

/**
 * Caret icon used to indicate a section is collapsed
 * or expanded; animates between states.
 *
 * Should be used within a <AccordionSummary /> component
 */
export const ResourceGroupSummaryIcon = styled(CaretSvg)`
  flex-shrink: 0;
  padding: ${SizeUnit(1 / 4)};
  transition: transform 300ms cubic-bezier(0.4, 0, 0.2, 1) 0ms; /* Copied from MUI accordion */

  .fillStd {
    fill: ${Color.grayLight};
  }
`

// Helper (non-Material UI) styles
/**
 * Common Tilt-specific styles for resource grouping;
 * should be used with a <AccordionSummary />.
 */
export const ResourceGroupSummaryMixin = `
  .MuiAccordionSummary-content {
    align-items: center;
    box-sizing: border-box;
    color: ${Color.white};
    display: flex;
    padding: ${SizeUnit(1 / 8)};
    width: 100%;

    &.Mui-expanded {
      ${ResourceGroupSummaryIcon} {
        transform: rotate(90deg);
      }
    }
  }
`
