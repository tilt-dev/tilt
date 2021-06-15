package containerupdate

func tarArgv() []string {
	return []string{"tar", "-C", "/", "-x", "-f", "-"}
}
