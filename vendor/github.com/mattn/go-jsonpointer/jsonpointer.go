package jsonpointer

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"unicode"
)

func parse(pointer string) ([]string, error) {
	pointer = strings.TrimLeftFunc(pointer, unicode.IsSpace)
	if !strings.HasPrefix(pointer, "/") {
		return nil, fmt.Errorf("invalid JSON pointer: %q", pointer)
	}
	tokens := strings.Split(pointer[1:], "/")
	if len(tokens) == 0 { //*|| len(tokens[0]) == 0 {
		return nil, fmt.Errorf("invalid JSON pointer: %q", pointer)
	}
	for i, token := range tokens {
		tokens[i] = strings.Replace(
			strings.Replace(token, "~1", "/", -1), "~0", "~", -1)
	}
	return tokens, nil
}

// Has return whether the obj has pointer.
func Has(obj interface{}, pointer string) (rv bool) {
	defer func() {
		if e := recover(); e != nil {
			rv = false
		}
	}()
	tokens, err := parse(pointer)
	if err != nil {
		return false
	}

	i := 0
	v := reflect.ValueOf(obj)
	if len(tokens) > 0 && tokens[0] != "" {
		for i < len(tokens) {
			for v.Kind() == reflect.Interface {
				v = v.Elem()
			}
			token := tokens[i]

			if n, err := strconv.Atoi(token); err == nil && isIndexed(v) {
				v = v.Index(n)
			} else {
				v = v.MapIndex(reflect.ValueOf(token))
			}
			i++
		}
		return v.IsValid()
	}
	return false
}

// Get return a value which is pointed with pointer on obj.
func Get(obj interface{}, pointer string) (rv interface{}, err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("invalid JSON pointer: %q: %v", pointer, e)
		}
	}()
	tokens, err := parse(pointer)
	if err != nil {
		return nil, err
	}

	i := 0
	v := reflect.ValueOf(obj)
	if len(tokens) > 0 && tokens[0] != "" {
		for i < len(tokens) {
			for v.Kind() == reflect.Interface {
				v = v.Elem()
			}
			token := tokens[i]
			if n, err := strconv.Atoi(token); err == nil && isIndexed(v) {
				v = v.Index(n)
			} else {
				v = v.MapIndex(reflect.ValueOf(token))
			}
			i++
		}
	}
	return v.Interface(), nil
}

// Set set a value which is pointed with pointer on obj.
func Set(obj interface{}, pointer string, value interface{}) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("invalid JSON pointer: %q: %v", pointer, e)
		}
	}()
	tokens, err := parse(pointer)
	if err != nil {
		return err
	}

	i := 0
	v := reflect.ValueOf(obj)
	var p reflect.Value
	var token string
	if len(tokens) > 0 && tokens[0] != "" {
		for i < len(tokens) {
			for v.Kind() == reflect.Interface {
				v = v.Elem()
			}
			p = v
			token = tokens[i]
			if n, err := strconv.Atoi(token); err == nil && isIndexed(v) {
				v = v.Index(n)
			} else {
				v = v.MapIndex(reflect.ValueOf(token))
			}
			i++
		}
		if p.Kind() == reflect.Map {
			p.SetMapIndex(reflect.ValueOf(token), reflect.ValueOf(value))
		} else {
			v.Set(reflect.ValueOf(value))
		}
		return nil
	}
	return fmt.Errorf("pointer should have element")
}

// Remove remove a value which is pointed with pointer on obj.
func Remove(obj interface{}, pointer string) (rv interface{}, err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("invalid JSON pointer: %q: %v", pointer, e)
		}
	}()
	tokens, err := parse(pointer)
	if err != nil {
		return nil, err
	}

	i := 0
	v := reflect.ValueOf(obj)
	var p, pp reflect.Value
	var token, ptoken string
	if len(tokens) > 0 && tokens[0] != "" {
		for i < len(tokens) {
			for v.Kind() == reflect.Interface {
				v = v.Elem()
			}
			pp = p
			p = v
			ptoken = token
			token = tokens[i]
			if n, err := strconv.Atoi(token); err == nil && isIndexed(v) {
				v = v.Index(n)
			} else {
				v = v.MapIndex(reflect.ValueOf(token))
			}
			i++
		}
	} else {
		return nil, fmt.Errorf("pointer should have element")
	}

	var nv reflect.Value
	if p.Kind() == reflect.Map {
		nv = reflect.MakeMap(p.Type())
		for _, mk := range p.MapKeys() {
			if mk.String() != token {
				nv.SetMapIndex(mk, p.MapIndex(mk))
			}
		}
	} else {
		nv = reflect.Zero(p.Type())
		n, _ := strconv.Atoi(token)
		for m := 0; m < p.Len(); m++ {
			if n != m {
				nv = reflect.Append(nv, p.Index(m))
			}
		}
	}

	if !pp.IsValid() {
		obj = nv.Interface()
	} else if pp.Kind() == reflect.Map {
		pp.SetMapIndex(reflect.ValueOf(ptoken), nv)
	} else {
		p.Set(reflect.ValueOf(nv))
	}
	return obj, nil
}

func isIndexed(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array:
		return true
	case reflect.Slice:
		return true
	}
	return false
}
