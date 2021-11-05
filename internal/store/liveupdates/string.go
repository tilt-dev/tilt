package liveupdates

func ContainerDisplayNames(containers []Container) []string {
	names := make([]string, 0, len(containers))
	for _, c := range containers {
		names = append(names, c.DisplayName())
	}
	return names
}
