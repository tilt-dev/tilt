// Helper methods for expvars that are initialized at startup.

package wmservice

import (
	"expvar"
	"fmt"
)

const ServiceNameVar = "serviceName"
const ServiceTagVar = "serviceTag"

func expvarString(varName string) string {
	v := expvar.Get(varName)
	if v == nil {
		return ""
	}

	vString, ok := v.(*expvar.String)
	if !ok {
		return ""
	}

	return vString.Value()
}

func ServiceName() (string, error) {
	serviceName := expvarString("serviceName")
	if serviceName == "" {
		return "", fmt.Errorf("serviceName expvar not set yet. Did you forget to create it in main.go?")
	}
	return serviceName, nil
}

func ServiceTag() (string, error) {
	serviceTag := expvarString("serviceTag")
	if serviceTag == "" {
		return "", fmt.Errorf("serviceTag expvar not set yet. Did you forget to create it in main.go?")
	}
	return serviceTag, nil
}
