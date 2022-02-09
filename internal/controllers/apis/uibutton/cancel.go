package uibutton

import "fmt"

func CancelButtonName(resourceName string) string {
	return fmt.Sprintf("%s-cancel", resourceName)
}
