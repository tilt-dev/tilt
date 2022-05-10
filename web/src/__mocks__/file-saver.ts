import FileSaver from "file-saver"

/**
 * Mock the FileSaver module, so its methods can be called and
 * spied on during tests.
 */
const fileSaver = jest.createMockFromModule<typeof FileSaver>("file-saver")

function saveAs(
  _data: Blob | string,
  _filename?: string,
  _disableAutoBOM?: boolean
): void {}

// @ts-ignore
fileSaver.saveAs = jest.fn(saveAs)

module.exports = fileSaver
