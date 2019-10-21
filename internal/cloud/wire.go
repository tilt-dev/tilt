package cloud

import "github.com/google/wire"

var WireSet = wire.NewSet(
	ProvideHttpClient,
	NewUsernameManager,
	NewSnapshotUploader,
	NewUpdateUploader)
