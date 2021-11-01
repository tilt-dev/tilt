package liveupdate

// Currently, LiveUpdate runs in one of two modes:
//
// 1) update-mode=auto, where files are synced into the container as we see them
// 2) update-mode=manual, where file changes are collected, but only synced
//    when the user clicks the trigger button.
//
// But triggers are not formally represented in the API. They're just a
// ConfigMap we hacked in.
//
// We should probably model this as a more full-featured UpdateStrategy,
// so that you can have more control over debouncing and number of retries.
//
// https://github.com/tilt-dev/tilt/issues/3606
// https://github.com/tilt-dev/tilt/issues/5054
//
// but for now we'll model this as an annotation.

const AnnotationUpdateMode = "tilt.dev/update-mode"
const UpdateModeAuto = "auto"
const UpdateModeManual = "manual"
