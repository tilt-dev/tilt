package sessions

import (
	"os"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

const DefaultSessionName = "Tiltfile"

var processStartTime = time.Now()
var processPID = int64(os.Getpid())

func FromTiltfile(tf *v1alpha1.Tiltfile, mode store.EngineMode) *v1alpha1.Session {
	s := &v1alpha1.Session{
		ObjectMeta: metav1.ObjectMeta{
			Name: DefaultSessionName,
		},
		Spec: v1alpha1.SessionSpec{
			TiltfilePath: tf.Spec.Path,
		},
		Status: v1alpha1.SessionStatus{
			PID:       processPID,
			StartTime: apis.NewMicroTime(processStartTime),
		},
	}

	// currently, manual + CI are the only supported modes; the apiserver will validate this field and reject
	// the object on creation if it doesn't conform, so there's no additional validation/error-handling here
	switch mode {
	case store.EngineModeUp:
		s.Spec.ExitCondition = v1alpha1.ExitConditionManual
	case store.EngineModeCI:
		s.Spec.ExitCondition = v1alpha1.ExitConditionCI
	}
	return s
}
