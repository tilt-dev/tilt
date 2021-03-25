// +build !ignore_autogenerated

/*
Copyright 2020 The Tilt Dev Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by deepcopy-gen. DO NOT EDIT.

package v1alpha1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ActiveBuild) DeepCopyInto(out *ActiveBuild) {
	*out = *in
	in.StartTime.DeepCopyInto(&out.StartTime)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ActiveBuild.
func (in *ActiveBuild) DeepCopy() *ActiveBuild {
	if in == nil {
		return nil
	}
	out := new(ActiveBuild)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *BuildState) DeepCopyInto(out *BuildState) {
	*out = *in
	if in.Pending != nil {
		in, out := &in.Pending, &out.Pending
		*out = new(PendingBuild)
		(*in).DeepCopyInto(*out)
	}
	if in.Active != nil {
		in, out := &in.Active, &out.Active
		*out = new(ActiveBuild)
		(*in).DeepCopyInto(*out)
	}
	if in.Terminated != nil {
		in, out := &in.Terminated, &out.Terminated
		*out = new(TerminatedBuild)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new BuildState.
func (in *BuildState) DeepCopy() *BuildState {
	if in == nil {
		return nil
	}
	out := new(BuildState)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Cmd) DeepCopyInto(out *Cmd) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Cmd.
func (in *Cmd) DeepCopy() *Cmd {
	if in == nil {
		return nil
	}
	out := new(Cmd)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Cmd) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CmdList) DeepCopyInto(out *CmdList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Cmd, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CmdList.
func (in *CmdList) DeepCopy() *CmdList {
	if in == nil {
		return nil
	}
	out := new(CmdList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *CmdList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CmdSpec) DeepCopyInto(out *CmdSpec) {
	*out = *in
	if in.Args != nil {
		in, out := &in.Args, &out.Args
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Env != nil {
		in, out := &in.Env, &out.Env
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.ReadinessProbe != nil {
		in, out := &in.ReadinessProbe, &out.ReadinessProbe
		*out = new(Probe)
		(*in).DeepCopyInto(*out)
	}
	if in.RestartOn != nil {
		in, out := &in.RestartOn, &out.RestartOn
		*out = new(RestartOnSpec)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CmdSpec.
func (in *CmdSpec) DeepCopy() *CmdSpec {
	if in == nil {
		return nil
	}
	out := new(CmdSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CmdStateRunning) DeepCopyInto(out *CmdStateRunning) {
	*out = *in
	in.StartedAt.DeepCopyInto(&out.StartedAt)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CmdStateRunning.
func (in *CmdStateRunning) DeepCopy() *CmdStateRunning {
	if in == nil {
		return nil
	}
	out := new(CmdStateRunning)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CmdStateTerminated) DeepCopyInto(out *CmdStateTerminated) {
	*out = *in
	in.StartedAt.DeepCopyInto(&out.StartedAt)
	in.FinishedAt.DeepCopyInto(&out.FinishedAt)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CmdStateTerminated.
func (in *CmdStateTerminated) DeepCopy() *CmdStateTerminated {
	if in == nil {
		return nil
	}
	out := new(CmdStateTerminated)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CmdStateWaiting) DeepCopyInto(out *CmdStateWaiting) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CmdStateWaiting.
func (in *CmdStateWaiting) DeepCopy() *CmdStateWaiting {
	if in == nil {
		return nil
	}
	out := new(CmdStateWaiting)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CmdStatus) DeepCopyInto(out *CmdStatus) {
	*out = *in
	if in.Waiting != nil {
		in, out := &in.Waiting, &out.Waiting
		*out = new(CmdStateWaiting)
		**out = **in
	}
	if in.Running != nil {
		in, out := &in.Running, &out.Running
		*out = new(CmdStateRunning)
		(*in).DeepCopyInto(*out)
	}
	if in.Terminated != nil {
		in, out := &in.Terminated, &out.Terminated
		*out = new(CmdStateTerminated)
		(*in).DeepCopyInto(*out)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CmdStatus.
func (in *CmdStatus) DeepCopy() *CmdStatus {
	if in == nil {
		return nil
	}
	out := new(CmdStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ExecAction) DeepCopyInto(out *ExecAction) {
	*out = *in
	if in.Command != nil {
		in, out := &in.Command, &out.Command
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ExecAction.
func (in *ExecAction) DeepCopy() *ExecAction {
	if in == nil {
		return nil
	}
	out := new(ExecAction)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FileEvent) DeepCopyInto(out *FileEvent) {
	*out = *in
	in.Time.DeepCopyInto(&out.Time)
	if in.SeenFiles != nil {
		in, out := &in.SeenFiles, &out.SeenFiles
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FileEvent.
func (in *FileEvent) DeepCopy() *FileEvent {
	if in == nil {
		return nil
	}
	out := new(FileEvent)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FileWatch) DeepCopyInto(out *FileWatch) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FileWatch.
func (in *FileWatch) DeepCopy() *FileWatch {
	if in == nil {
		return nil
	}
	out := new(FileWatch)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *FileWatch) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FileWatchList) DeepCopyInto(out *FileWatchList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]FileWatch, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FileWatchList.
func (in *FileWatchList) DeepCopy() *FileWatchList {
	if in == nil {
		return nil
	}
	out := new(FileWatchList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *FileWatchList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FileWatchSpec) DeepCopyInto(out *FileWatchSpec) {
	*out = *in
	if in.WatchedPaths != nil {
		in, out := &in.WatchedPaths, &out.WatchedPaths
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Ignores != nil {
		in, out := &in.Ignores, &out.Ignores
		*out = make([]IgnoreDef, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FileWatchSpec.
func (in *FileWatchSpec) DeepCopy() *FileWatchSpec {
	if in == nil {
		return nil
	}
	out := new(FileWatchSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FileWatchStatus) DeepCopyInto(out *FileWatchStatus) {
	*out = *in
	in.MonitorStartTime.DeepCopyInto(&out.MonitorStartTime)
	in.LastEventTime.DeepCopyInto(&out.LastEventTime)
	if in.FileEvents != nil {
		in, out := &in.FileEvents, &out.FileEvents
		*out = make([]FileEvent, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FileWatchStatus.
func (in *FileWatchStatus) DeepCopy() *FileWatchStatus {
	if in == nil {
		return nil
	}
	out := new(FileWatchStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HTTPGetAction) DeepCopyInto(out *HTTPGetAction) {
	*out = *in
	if in.HTTPHeaders != nil {
		in, out := &in.HTTPHeaders, &out.HTTPHeaders
		*out = make([]HTTPHeader, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HTTPGetAction.
func (in *HTTPGetAction) DeepCopy() *HTTPGetAction {
	if in == nil {
		return nil
	}
	out := new(HTTPGetAction)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *HTTPHeader) DeepCopyInto(out *HTTPHeader) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new HTTPHeader.
func (in *HTTPHeader) DeepCopy() *HTTPHeader {
	if in == nil {
		return nil
	}
	out := new(HTTPHeader)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Handler) DeepCopyInto(out *Handler) {
	*out = *in
	if in.Exec != nil {
		in, out := &in.Exec, &out.Exec
		*out = new(ExecAction)
		(*in).DeepCopyInto(*out)
	}
	if in.HTTPGet != nil {
		in, out := &in.HTTPGet, &out.HTTPGet
		*out = new(HTTPGetAction)
		(*in).DeepCopyInto(*out)
	}
	if in.TCPSocket != nil {
		in, out := &in.TCPSocket, &out.TCPSocket
		*out = new(TCPSocketAction)
		**out = **in
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Handler.
func (in *Handler) DeepCopy() *Handler {
	if in == nil {
		return nil
	}
	out := new(Handler)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *IgnoreDef) DeepCopyInto(out *IgnoreDef) {
	*out = *in
	if in.Patterns != nil {
		in, out := &in.Patterns, &out.Patterns
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new IgnoreDef.
func (in *IgnoreDef) DeepCopy() *IgnoreDef {
	if in == nil {
		return nil
	}
	out := new(IgnoreDef)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PendingBuild) DeepCopyInto(out *PendingBuild) {
	*out = *in
	in.TriggerTime.DeepCopyInto(&out.TriggerTime)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PendingBuild.
func (in *PendingBuild) DeepCopy() *PendingBuild {
	if in == nil {
		return nil
	}
	out := new(PendingBuild)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PodLogStream) DeepCopyInto(out *PodLogStream) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PodLogStream.
func (in *PodLogStream) DeepCopy() *PodLogStream {
	if in == nil {
		return nil
	}
	out := new(PodLogStream)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PodLogStream) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PodLogStreamList) DeepCopyInto(out *PodLogStreamList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]PodLogStream, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PodLogStreamList.
func (in *PodLogStreamList) DeepCopy() *PodLogStreamList {
	if in == nil {
		return nil
	}
	out := new(PodLogStreamList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *PodLogStreamList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PodLogStreamSpec) DeepCopyInto(out *PodLogStreamSpec) {
	*out = *in
	if in.SinceTime != nil {
		in, out := &in.SinceTime, &out.SinceTime
		*out = (*in).DeepCopy()
	}
	if in.OnlyContainers != nil {
		in, out := &in.OnlyContainers, &out.OnlyContainers
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.IgnoreContainers != nil {
		in, out := &in.IgnoreContainers, &out.IgnoreContainers
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PodLogStreamSpec.
func (in *PodLogStreamSpec) DeepCopy() *PodLogStreamSpec {
	if in == nil {
		return nil
	}
	out := new(PodLogStreamSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *PodLogStreamStatus) DeepCopyInto(out *PodLogStreamStatus) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new PodLogStreamStatus.
func (in *PodLogStreamStatus) DeepCopy() *PodLogStreamStatus {
	if in == nil {
		return nil
	}
	out := new(PodLogStreamStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Probe) DeepCopyInto(out *Probe) {
	*out = *in
	in.Handler.DeepCopyInto(&out.Handler)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Probe.
func (in *Probe) DeepCopy() *Probe {
	if in == nil {
		return nil
	}
	out := new(Probe)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ResourceState) DeepCopyInto(out *ResourceState) {
	*out = *in
	if in.Build != nil {
		in, out := &in.Build, &out.Build
		*out = new(BuildState)
		(*in).DeepCopyInto(*out)
	}
	out.Runtime = in.Runtime
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ResourceState.
func (in *ResourceState) DeepCopy() *ResourceState {
	if in == nil {
		return nil
	}
	out := new(ResourceState)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RestartOnSpec) DeepCopyInto(out *RestartOnSpec) {
	*out = *in
	if in.FileWatches != nil {
		in, out := &in.FileWatches, &out.FileWatches
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RestartOnSpec.
func (in *RestartOnSpec) DeepCopy() *RestartOnSpec {
	if in == nil {
		return nil
	}
	out := new(RestartOnSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RuntimeState) DeepCopyInto(out *RuntimeState) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RuntimeState.
func (in *RuntimeState) DeepCopy() *RuntimeState {
	if in == nil {
		return nil
	}
	out := new(RuntimeState)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TCPSocketAction) DeepCopyInto(out *TCPSocketAction) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TCPSocketAction.
func (in *TCPSocketAction) DeepCopy() *TCPSocketAction {
	if in == nil {
		return nil
	}
	out := new(TCPSocketAction)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TerminatedBuild) DeepCopyInto(out *TerminatedBuild) {
	*out = *in
	in.StartTime.DeepCopyInto(&out.StartTime)
	in.FinishTime.DeepCopyInto(&out.FinishTime)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TerminatedBuild.
func (in *TerminatedBuild) DeepCopy() *TerminatedBuild {
	if in == nil {
		return nil
	}
	out := new(TerminatedBuild)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TiltRun) DeepCopyInto(out *TiltRun) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TiltRun.
func (in *TiltRun) DeepCopy() *TiltRun {
	if in == nil {
		return nil
	}
	out := new(TiltRun)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *TiltRun) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TiltRunList) DeepCopyInto(out *TiltRunList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]TiltRun, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TiltRunList.
func (in *TiltRunList) DeepCopy() *TiltRunList {
	if in == nil {
		return nil
	}
	out := new(TiltRunList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *TiltRunList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TiltRunSpec) DeepCopyInto(out *TiltRunSpec) {
	*out = *in
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TiltRunSpec.
func (in *TiltRunSpec) DeepCopy() *TiltRunSpec {
	if in == nil {
		return nil
	}
	out := new(TiltRunSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TiltRunStatus) DeepCopyInto(out *TiltRunStatus) {
	*out = *in
	in.StartTime.DeepCopyInto(&out.StartTime)
	in.Tiltfile.DeepCopyInto(&out.Tiltfile)
	if in.Resources != nil {
		in, out := &in.Resources, &out.Resources
		*out = make([]ResourceState, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TiltRunStatus.
func (in *TiltRunStatus) DeepCopy() *TiltRunStatus {
	if in == nil {
		return nil
	}
	out := new(TiltRunStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TiltfileState) DeepCopyInto(out *TiltfileState) {
	*out = *in
	in.Build.DeepCopyInto(&out.Build)
	return
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TiltfileState.
func (in *TiltfileState) DeepCopy() *TiltfileState {
	if in == nil {
		return nil
	}
	out := new(TiltfileState)
	in.DeepCopyInto(out)
	return out
}
