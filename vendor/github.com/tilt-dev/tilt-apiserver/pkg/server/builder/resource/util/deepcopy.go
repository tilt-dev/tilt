package util

import (
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
)

const deepCopyIntoMethodName = "DeepCopyInto"

// DeepCopy deep-copies object from src to dst. The objects requires the go type to properly
// implement DeepCopyInto method other an error will be returned.
func DeepCopy(src, dst runtime.Object) error {
	deepCopyIntoMethod := reflect.ValueOf(src).MethodByName(deepCopyIntoMethodName)
	if !deepCopyIntoMethod.IsValid() {
		return fmt.Errorf("no %v method found on type %v", deepCopyIntoMethod, reflect.TypeOf(src).String())
	}
	if err := checkSignature(reflect.TypeOf(src), deepCopyIntoMethod); err != nil {
		return err
	}
	deepCopyIntoMethod.Call([]reflect.Value{reflect.ValueOf(dst)})
	return nil
}

func checkSignature(src reflect.Type, method reflect.Value) error {
	if method.Type().NumIn() != 1 {
		return fmt.Errorf("invalid number of arguments for method %v upon %v, should be 1", deepCopyIntoMethodName, src)
	}
	if !method.Type().In(0).AssignableTo(src) {
		return fmt.Errorf("invalid type of arguments[0] for method %v upon %v, expected %v", deepCopyIntoMethodName, src, src)
	}
	if method.Type().NumOut() != 0 {
		return fmt.Errorf("method %v upon %v should not have return values", deepCopyIntoMethodName, src)
	}
	return nil
}
