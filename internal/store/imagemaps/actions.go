package imagemaps

import "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

type ImageMapUpsertAction struct {
	ImageMap *v1alpha1.ImageMap
}

func NewImageMapUpsertAction(obj *v1alpha1.ImageMap) ImageMapUpsertAction {
	return ImageMapUpsertAction{ImageMap: obj}
}

func (ImageMapUpsertAction) Action() {}

type ImageMapDeleteAction struct {
	Name string
}

func NewImageMapDeleteAction(n string) ImageMapDeleteAction {
	return ImageMapDeleteAction{Name: n}
}

func (ImageMapDeleteAction) Action() {}
