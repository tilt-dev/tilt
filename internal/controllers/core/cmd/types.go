package cmd

import "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

type Cmd = v1alpha1.Cmd
type CmdSpec = v1alpha1.CmdSpec
type CmdStatus = v1alpha1.CmdStatus
type CmdStateRunning = v1alpha1.CmdStateRunning
type CmdStateTerminated = v1alpha1.CmdStateTerminated

const LabelManifest = v1alpha1.LabelManifest
