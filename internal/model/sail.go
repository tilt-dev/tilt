package model

import "net/url"

const DefaultSailPort = 10450

type SailPort int
type SailURL url.URL

func (u SailURL) String() string {
	url := (*url.URL)(&u)
	return url.String()
}

func (u SailURL) Empty() bool {
	return SailURL{} == u
}
