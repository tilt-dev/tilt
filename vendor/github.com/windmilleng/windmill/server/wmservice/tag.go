package wmservice

const hermeticTag = "hermetic"

func IsLocalService(tag string) bool {
	return tag == hermeticTag
}

func IsCloudService(tag string) bool {
	return !IsLocalService(tag)
}
