package model

type SecretSettings struct {
	ScrubSecrets bool // whether to scrub secrets in logs
}

func DefaultSecretSettings() SecretSettings {
	return SecretSettings{ScrubSecrets: true}
}
