package store

import (
	"fmt"
	"time"

	"github.com/windmilleng/wmclient/pkg/analytics"
	v1 "k8s.io/api/core/v1"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"
	"github.com/windmilleng/tilt/pkg/model/logstore"
)

type ErrorAction struct {
	Error error
}

func (ErrorAction) Action() {}

func NewErrorAction(err error) ErrorAction {
	return ErrorAction{Error: err}
}

type LogAction struct {
	mn        model.ManifestName
	spanID    logstore.SpanID
	timestamp time.Time
	fields    logger.Fields
	msg       []byte
	level     logger.Level
}

func (LogAction) Action() {}

func (le LogAction) ManifestName() model.ManifestName {
	return le.mn
}

func (le LogAction) Level() logger.Level {
	return le.level
}

func (le LogAction) Time() time.Time {
	return le.timestamp
}

func (le LogAction) Fields() logger.Fields {
	return le.fields
}

func (le LogAction) Message() []byte {
	return le.msg
}

func (le LogAction) SpanID() logstore.SpanID {
	return le.spanID
}

func (le LogAction) String() string {
	return fmt.Sprintf("manifest: %s, spanID: %s, msg: %q", le.mn, le.spanID, le.msg)
}

func NewLogAction(mn model.ManifestName, spanID logstore.SpanID, level logger.Level, fields logger.Fields, b []byte) LogAction {
	return LogAction{
		mn:        mn,
		spanID:    spanID,
		level:     level,
		timestamp: time.Now(),
		msg:       append([]byte{}, b...),
		fields:    fields,
	}
}

func NewGlobalLogAction(level logger.Level, b []byte) LogAction {
	return LogAction{
		mn:        "",
		spanID:    "",
		level:     level,
		timestamp: time.Now(),
		msg:       append([]byte{}, b...),
	}
}

type K8sEventAction struct {
	Event        *v1.Event
	ManifestName model.ManifestName
}

func (K8sEventAction) Action() {}

func NewK8sEventAction(event *v1.Event, manifestName model.ManifestName) K8sEventAction {
	return K8sEventAction{event, manifestName}
}

func (kEvt K8sEventAction) ToLogAction(mn model.ManifestName) LogAction {
	msg := fmt.Sprintf("[K8s EVENT: %s] %s\n",
		objRefHumanReadable(kEvt.Event.InvolvedObject), kEvt.Event.Message)

	return LogAction{
		mn:        mn,
		spanID:    logstore.SpanID(fmt.Sprintf("events:%s", mn)),
		level:     logger.InfoLvl,
		timestamp: kEvt.Event.LastTimestamp.Time,
		msg:       []byte(msg),
	}
}

func objRefHumanReadable(obj v1.ObjectReference) string {
	s := fmt.Sprintf("%s %s", obj.Kind, obj.Name)
	if obj.Namespace != "" {
		s += fmt.Sprintf(" (ns: %s)", obj.Namespace)
	}
	return s
}

type AnalyticsUserOptAction struct {
	Opt analytics.Opt
}

func (AnalyticsUserOptAction) Action() {}

type AnalyticsNudgeSurfacedAction struct{}

func (AnalyticsNudgeSurfacedAction) Action() {}

type TiltCloudUserLookedUpAction struct {
	Found                    bool
	Username                 string
	IsPostRegistrationLookup bool
}

func (TiltCloudUserLookedUpAction) Action() {}

type UserStartedTiltCloudRegistrationAction struct{}

func (UserStartedTiltCloudRegistrationAction) Action() {}

// The user can indicate "yes, I know the pod restarted N times, stop showing me"
type PodResetRestartsAction struct {
	PodID           k8s.PodID
	ManifestName    model.ManifestName
	VisibleRestarts int
}

func NewPodResetRestartsAction(podID k8s.PodID, mn model.ManifestName, visibleRestarts int) PodResetRestartsAction {
	return PodResetRestartsAction{
		PodID:           podID,
		ManifestName:    mn,
		VisibleRestarts: visibleRestarts,
	}
}

func (PodResetRestartsAction) Action() {}

type PanicAction struct {
	Err error
}

func (PanicAction) Action() {}
