package cloud

import "github.com/google/wire"

var WireSet = wire.NewSet(
	ProvideAddress,
	ProvideHttpClient,
	NewUsernameManager,
	NewSnapshotUploader,
	NewUpdateUploader)
