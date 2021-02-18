package starkit

// A dummy interface that allows to pass around additional info for sending to
// analytics at the end of Tiltfile execution.
type AnalyticsInfo interface {
	AnalyticsInfo()
}

// Info on use of Extensions.
type ExtensionsAnalyticsInfo struct {
	ExtensionsLoaded map[string]bool
}

func NewExtensionsAnalyticsInfo() ExtensionsAnalyticsInfo {
	return ExtensionsAnalyticsInfo{
		ExtensionsLoaded: make(map[string]bool),
	}
}

func (ExtensionsAnalyticsInfo) AnalyticsInfo() {}
