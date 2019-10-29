package analytics

type Option func(a *remoteAnalytics)

// Sets global tags for every report.
func WithGlobalTags(tags map[string]string) Option {
	return Option(func(a *remoteAnalytics) {
		for k, v := range tags {
			a.globalTags[k] = v
		}
	})
}

// Sets the URL to report to. Defaults to https://events.windmill.build/report
func WithReportURL(url string) Option {
	return Option(func(a *remoteAnalytics) {
		a.url = url
	})
}

// Sets the HTTP client. Defaults to golang http client.
func WithHTTPClient(client HTTPClient) Option {
	return Option(func(a *remoteAnalytics) {
		a.cli = client
	})
}

// Sets the UserID. Defaults to a user ID based on a hash of a machine identifier.
func WithUserID(userID string) Option {
	return Option(func(a *remoteAnalytics) {
		a.globalTags[TagUser] = userID
	})
}

func WithMachineID(machineID string) Option {
	return Option(func(a *remoteAnalytics) {
		a.globalTags[TagMachine] = machineID
	})
}

// Sets whether the analytics client is enabled. Defaults to checking
// the users' opt-in setting in ~/.windmill
func WithEnabled(enabled bool) Option {
	return Option(func(a *remoteAnalytics) {
		a.enabled = enabled
	})
}

// Sets the logger where errors are printed.
// Defaults to printing to the default logger with the prefix "[analytics] ".
func WithLogger(logger Logger) Option {
	return Option(func(a *remoteAnalytics) {
		a.logger = logger
	})
}
