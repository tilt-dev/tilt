package confutil

import "os"

func sudoer(_ string) *chowner {
	return nil
}

func fileOwner(_ os.FileInfo) *chowner {
	return nil
}
