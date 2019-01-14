package analytics

type Option func(a *remoteAnalytics)

func WithGlobalTags(tags map[string]string) Option {
	return Option(func(a *remoteAnalytics) {
		for k, v := range tags {
			a.globalTags[k] = v
		}
	})
}
