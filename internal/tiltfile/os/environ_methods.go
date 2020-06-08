package os

/**!
 Loosely adapted from
 https://github.com/google/starlark-go
 Forked the code for dict() methods and made them work for environ.

Copyright (c) 2017 The Bazel Authors.  All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are
met:

1. Redistributions of source code must retain the above copyright
   notice, this list of conditions and the following disclaimer.

2. Redistributions in binary form must reproduce the above copyright
   notice, this list of conditions and the following disclaimer in the
   documentation and/or other materials provided with the
   distribution.

3. Neither the name of the copyright holder nor the names of its
   contributors may be used to endorse or promote products derived
   from this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
"AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
*/

import (
	"fmt"
	"sort"

	"go.starlark.net/starlark"
)

type builtinMethod func(b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error)

// methods of built-in types
// https://github.com/google/starlark-go/blob/master/doc/spec.md#built-in-methods
var (
	environMethods = map[string]builtinMethod{
		"clear":      environ_clear,
		"get":        environ_get,
		"items":      environ_items,
		"keys":       environ_keys,
		"pop":        environ_pop,
		"popitem":    environ_popitem,
		"setdefault": environ_setdefault,
		"update":     environ_update,
		"values":     environ_values,
	}
)

// https://github.com/google/starlark-go/blob/master/doc/spec.md#dict·get
func environ_get(b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var key, dflt starlark.Value
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 1, &key, &dflt); err != nil {
		return nil, err
	}
	if v, ok, err := b.Receiver().(Environ).Get(key); err != nil {
		return nil, nameErr(b, err)
	} else if ok {
		return v, nil
	} else if dflt != nil {
		return dflt, nil
	}
	return starlark.None, nil
}

// https://github.com/google/starlark-go/blob/master/doc/spec.md#dict·clear
func environ_clear(b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 0); err != nil {
		return nil, err
	}
	return starlark.None, b.Receiver().(Environ).Clear()
}

// https://github.com/google/starlark-go/blob/master/doc/spec.md#dict·items
func environ_items(b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 0); err != nil {
		return nil, err
	}
	items := b.Receiver().(Environ).Items()
	res := make([]starlark.Value, len(items))
	for i, item := range items {
		res[i] = item // convert [2]starlark.Value to starlark.Value
	}
	return starlark.NewList(res), nil
}

// https://github.com/google/starlark-go/blob/master/doc/spec.md#dict·keys
func environ_keys(b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 0); err != nil {
		return nil, err
	}
	return starlark.NewList(b.Receiver().(Environ).Keys()), nil
}

// https://github.com/google/starlark-go/blob/master/doc/spec.md#dict·pop
func environ_pop(b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var k, d starlark.Value
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 1, &k, &d); err != nil {
		return nil, err
	}
	if v, found, err := b.Receiver().(Environ).Delete(k); err != nil {
		return nil, nameErr(b, err) // environ is frozen or key is unhashable
	} else if found {
		return v, nil
	} else if d != nil {
		return d, nil
	}
	return nil, nameErr(b, "missing key")
}

// https://github.com/google/starlark-go/blob/master/doc/spec.md#dict·popitem
func environ_popitem(b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 0); err != nil {
		return nil, err
	}
	recv := b.Receiver().(Environ)
	keys := recv.Keys()
	if len(keys) == 0 {
		return nil, nameErr(b, "empty environ")
	}
	k := keys[0]
	v, _, err := recv.Delete(k)
	if err != nil {
		return nil, nameErr(b, err) // environ is frozen
	}
	return starlark.Tuple{k, v}, nil
}

// https://github.com/google/starlark-go/blob/master/doc/spec.md#dict·setdefault
func environ_setdefault(b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var key, dflt starlark.Value = nil, starlark.None
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 1, &key, &dflt); err != nil {
		return nil, err
	}
	environ := b.Receiver().(Environ)
	if v, ok, err := environ.Get(key); err != nil {
		return nil, nameErr(b, err)
	} else if ok {
		return v, nil
	} else if err := environ.SetKey(key, dflt); err != nil {
		return nil, nameErr(b, err)
	} else {
		return dflt, nil
	}
}

// https://github.com/google/starlark-go/blob/master/doc/spec.md#dict·update
func environ_update(b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) > 1 {
		return nil, fmt.Errorf("update: got %d arguments, want at most 1", len(args))
	}
	if err := updateEnviron(b.Receiver().(Environ), args, kwargs); err != nil {
		return nil, fmt.Errorf("update: %v", err)
	}
	return starlark.None, nil
}

// Common implementation of builtin dict function and dict.update method.
// Precondition: len(updates) == 0 or 1.
func updateEnviron(dict Environ, updates starlark.Tuple, kwargs []starlark.Tuple) error {
	if len(updates) == 1 {
		switch updates := updates[0].(type) {
		case starlark.IterableMapping:
			// Iterate over dict's key/value pairs, not just keys.
			for _, item := range updates.Items() {
				if err := dict.SetKey(item[0], item[1]); err != nil {
					return err // dict is frozen
				}
			}
		default:
			// all other sequences
			iter := starlark.Iterate(updates)
			if iter == nil {
				return fmt.Errorf("got %s, want iterable", updates.Type())
			}
			defer iter.Done()
			var pair starlark.Value
			for i := 0; iter.Next(&pair); i++ {
				iter2 := starlark.Iterate(pair)
				if iter2 == nil {
					return fmt.Errorf("dictionary update sequence element #%d is not iterable (%s)", i, pair.Type())

				}
				defer iter2.Done()
				len := starlark.Len(pair)
				if len < 0 {
					return fmt.Errorf("dictionary update sequence element #%d has unknown length (%s)", i, pair.Type())
				} else if len != 2 {
					return fmt.Errorf("dictionary update sequence element #%d has length %d, want 2", i, len)
				}
				var k, v starlark.Value
				iter2.Next(&k)
				iter2.Next(&v)
				if err := dict.SetKey(k, v); err != nil {
					return err
				}
			}
		}
	}

	// Then add the kwargs.
	before := dict.Len()
	for _, pair := range kwargs {
		if err := dict.SetKey(pair[0], pair[1]); err != nil {
			return err // dict is frozen
		}
	}
	// In the common case, each kwarg will add another dict entry.
	// If that's not so, check whether it is because there was a duplicate kwarg.
	if dict.Len() < before+len(kwargs) {
		keys := make(map[starlark.String]bool, len(kwargs))
		for _, kv := range kwargs {
			k := kv[0].(starlark.String)
			if keys[k] {
				return fmt.Errorf("duplicate keyword arg: %v", k)
			}
			keys[k] = true
		}
	}

	return nil
}

// https://github.com/google/starlark-go/blob/master/doc/spec.md#dict·update
func environ_values(b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 0); err != nil {
		return nil, err
	}
	items := b.Receiver().(Environ).Items()
	res := make([]starlark.Value, len(items))
	for i, item := range items {
		res[i] = item[1]
	}
	return starlark.NewList(res), nil
}

func builtinAttr(recv starlark.Value, name string, methods map[string]builtinMethod) (starlark.Value, error) {
	method := methods[name]
	if method == nil {
		return nil, nil // no such method
	}

	// Allocate a closure over 'method'.
	impl := func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		return method(b, args, kwargs)
	}
	return starlark.NewBuiltin(name, impl).BindReceiver(recv), nil
}

func builtinAttrNames(methods map[string]builtinMethod) []string {
	names := make([]string, 0, len(methods))
	for name := range methods {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// nameErr returns an error message of the form "name: msg"
// where name is b.Name() and msg is a string or error.
func nameErr(b *starlark.Builtin, msg interface{}) error {
	return fmt.Errorf("%s: %v", b.Name(), msg)
}
