package cloud

import "github.com/google/wire"

var WireSet = wire.NewSet(
	ProvideHttpClient,
	NewStatusManager,
	NewSnapshotUploader,
	NewUpdateUploader)
