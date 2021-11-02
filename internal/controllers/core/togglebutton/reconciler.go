package togglebutton

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/tilt-dev/tilt/internal/controllers/indexer"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

const (
	actionUIInputName = "action"
	turnOnInputValue  = "on"
	turnOffInputValue = "off"
)

type Reconciler struct {
	ctrlClient            ctrlclient.Client
	indexer               *indexer.Indexer
	lastClickProcessTimes map[string]time.Time
}

func (r *Reconciler) CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error) {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ToggleButton{}).
		Watches(&source.Kind{Type: &v1alpha1.ConfigMap{}},
			handler.EnqueueRequestsFromMapFunc(r.indexer.Enqueue)).
		Owns(&v1alpha1.UIButton{})

	return b, nil
}

func NewReconciler(ctrlClient ctrlclient.Client, scheme *runtime.Scheme) *Reconciler {
	return &Reconciler{
		ctrlClient:            ctrlClient,
		indexer:               indexer.NewIndexer(scheme, indexToggleButton),
		lastClickProcessTimes: make(map[string]time.Time),
	}
}

func indexToggleButton(obj client.Object) []indexer.Key {
	var result []indexer.Key
	toggleButton := obj.(*v1alpha1.ToggleButton)
	bGVK := v1alpha1.SchemeGroupVersion.WithKind("ConfigMap")

	if toggleButton != nil {
		if toggleButton.Spec.StateSource.ConfigMap != nil {
			result = append(result, indexer.Key{
				Name: types.NamespacedName{Name: toggleButton.Spec.StateSource.ConfigMap.Name},
				GVK:  bGVK,
			})
		}
	}
	return result
}

func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	nn := request.NamespacedName

	tb := &v1alpha1.ToggleButton{}
	err := r.ctrlClient.Get(ctx, nn, tb)
	r.indexer.OnReconcile(nn, tb)
	if client.IgnoreNotFound(err) != nil {
		return ctrl.Result{}, err
	}

	if apierrors.IsNotFound(err) || !tb.ObjectMeta.DeletionTimestamp.IsZero() {
		err := r.managedOwnedUIButton(ctx, nn, nil, false)
		return ctrl.Result{}, err
	}

	origError := tb.Status.Error
	// clear the error - if its conditions still apply, it will get re-set
	tb.Status.Error = ""

	err = r.processClick(ctx, tb)
	if err != nil {
		return ctrl.Result{}, err
	}

	isOn, err := r.isOn(ctx, tb)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.managedOwnedUIButton(ctx, nn, tb, isOn)
	if err != nil {
		return ctrl.Result{}, err
	}

	if origError != tb.Status.Error {
		err := r.ctrlClient.Status().Update(ctx, tb)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, err
}

func (r *Reconciler) processClick(ctx context.Context, tb *v1alpha1.ToggleButton) error {
	uiButton := v1alpha1.UIButton{}
	err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: uibuttonName(tb.Name)}, &uiButton)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		} else {
			return err
		}
	}

	// if there's a new click, pass the new value through to the ConfigMap
	if uiButton.Status.LastClickedAt.After(r.lastClickProcessTimes[tb.Name]) {
		foundInput := false
		var isOn bool
		for _, input := range uiButton.Status.Inputs {
			if input.Name == actionUIInputName {
				if input.Hidden == nil {
					tb.Status.Error = fmt.Sprintf(
						"button %q input %q was not of type 'Hidden'",
						uiButton.Name,
						input.Name,
					)
					return nil
				}
				switch input.Hidden.Value {
				case turnOnInputValue:
					isOn = true
				case turnOffInputValue:
					isOn = false
				default:
					tb.Status.Error = fmt.Sprintf("button %q input %q had unexpected value %q", uiButton.Name, input.Name, input.Hidden.Value)
					return nil
				}
				foundInput = true
				break
			}
		}

		if !foundInput {
			tb.Status.Error = fmt.Sprintf("UIButton %q does not have an input named %q", uiButton.Name, actionUIInputName)
			return nil
		}

		ss := tb.Spec.StateSource.ConfigMap
		if ss == nil {
			tb.Status.Error = "Spec.StateSource.ConfigMap is nil"
			return nil
		}
		var cm v1alpha1.ConfigMap
		err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: ss.Name}, &cm)
		if err != nil {
			return errors.Wrap(err, "fetching ToggleButton ConfigMap")
		}

		var newValue string
		if isOn {
			newValue = ss.OnValue
		} else {
			newValue = ss.OffValue
		}

		if cm.Data == nil {
			cm.Data = make(map[string]string)
		}

		currentValue, ok := cm.Data[ss.Key]

		if !ok || currentValue != newValue {
			cm.Data[ss.Key] = newValue
			err := r.ctrlClient.Update(ctx, &cm)
			if err != nil {
				return errors.Wrap(err, "updating ConfigMap with ToggleButton value")
			}
		}

		r.lastClickProcessTimes[tb.Name] = time.Now()
	}

	return nil
}

func (r *Reconciler) isOn(ctx context.Context, tb *v1alpha1.ToggleButton) (bool, error) {
	isOn := tb.Spec.DefaultOn
	ss := tb.Spec.StateSource.ConfigMap
	if ss == nil {
		tb.Status.Error = "Spec.StateSource.ConfigMap is nil"
		return isOn, nil
	}
	var cm v1alpha1.ConfigMap
	err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: ss.Name}, &cm)
	if client.IgnoreNotFound(err) != nil {
		return false, errors.Wrapf(err, "fetching ToggleButton %q ConfigMap %q", tb.Name, ss.Name)
	}

	if apierrors.IsNotFound(err) {
		tb.Status.Error = fmt.Sprintf("no such ConfigMap %q", ss.Name)
		return isOn, nil
	}

	if cm.Data != nil {
		cmVal, ok := cm.Data[ss.Key]
		if ok {
			switch cmVal {
			case ss.OnValue:
				isOn = true
			case ss.OffValue:
				isOn = false
			default:
				tb.Status.Error = fmt.Sprintf(
					"ConfigMap %q key %q has unknown value %q. expected %q or %q",
					ss.Name,
					ss.Key,
					cmVal,
					ss.OnValue,
					ss.OffValue)
				return isOn, nil
			}
		}
	}

	return isOn, nil
}

func (r *Reconciler) managedOwnedUIButton(ctx context.Context, nn types.NamespacedName, tb *v1alpha1.ToggleButton, isOn bool) error {
	b := &v1alpha1.UIButton{ObjectMeta: metav1.ObjectMeta{Name: uibuttonName(nn.Name), Namespace: nn.Namespace}}

	if tb == nil {
		err := r.ctrlClient.Delete(ctx, b)
		return ctrlclient.IgnoreNotFound(err)
	}

	_, err := ctrl.CreateOrUpdate(ctx, r.ctrlClient, b, func() error {
		return r.configureUIButton(b, isOn, tb)
	})
	if err != nil {
		return errors.Wrapf(err, "upserting ToggleButton %q's UIButton", tb.Name)
	}

	return nil
}

func uibuttonName(tbName string) string {
	return fmt.Sprintf("toggle-%s", tbName)
}

func (r *Reconciler) configureUIButton(b *v1alpha1.UIButton, isOn bool, tb *v1alpha1.ToggleButton) error {
	var stateSpec v1alpha1.ToggleButtonStateSpec
	var value string
	if isOn {
		stateSpec = tb.Spec.On
		value = turnOffInputValue
	} else {
		stateSpec = tb.Spec.Off
		value = turnOnInputValue
	}

	b.Spec = v1alpha1.UIButtonSpec{
		Location:             tb.Spec.Location,
		Text:                 stateSpec.Text,
		IconName:             stateSpec.IconName,
		IconSVG:              stateSpec.IconSVG,
		RequiresConfirmation: stateSpec.RequiresConfirmation,
		Inputs: []v1alpha1.UIInputSpec{
			{
				Name:   actionUIInputName,
				Hidden: &v1alpha1.UIHiddenInputSpec{Value: value},
			},
		},
		ButtonType: tb.Spec.ButtonType,
	}

	err := controllerutil.SetControllerReference(tb, b, r.ctrlClient.Scheme())
	if err != nil {
		return errors.Wrapf(err, "setting ToggleButton %q's UIButton's controller reference", tb.Name)
	}

	return nil
}
