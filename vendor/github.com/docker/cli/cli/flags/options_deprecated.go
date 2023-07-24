package flags

// CommonOptions are options common to both the client and the daemon.
//
// Deprecated: use [ClientOptions].
type CommonOptions = ClientOptions

// NewCommonOptions returns a new CommonOptions
//
// Deprecated: use [NewClientOptions].
var NewCommonOptions = NewClientOptions
