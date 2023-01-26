// .storybook/main.js

module.exports = {
  stories: ['../src/**/*.stories.tsx'],
  core: {
    builder: 'webpack5'
  },
  addons: [
    '@storybook/addon-essentials',
    '@storybook/preset-create-react-app'
  ],
  typescript: {
    // https://github.com/styleguidist/react-docgen-typescript/issues/356
    reactDocgen: 'none',
  },
};
