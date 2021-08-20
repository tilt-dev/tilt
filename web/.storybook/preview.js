import "../src/index.scss"
import { StylesProvider } from '@material-ui/core/styles';

export const parameters = {
  options: {
    storySort: {
      order: ['New UI', ['OverviewTablePane', 'OverviewResourcePane', 'Overview', 'Log View', 'Shared', '_To Review'], 'Legacy UI'], 
    },
  },
};

export const decorators = [
  (Story) => (
    <>
      {/* required for MUI <Icon> */}
      <link
          rel="stylesheet"
          href="https://fonts.googleapis.com/icon?family=Material+Icons"
      />
      { /* https://material-ui.com/guides/interoperability/#controlling-priority-3 */ }
      <StylesProvider injectFirst>
        <Story />
      </StylesProvider>
    </>
  ),
]
