// Package xlsx builds Excel workbooks from calculator API responses. It
// is generic across all calculators: it walks whatever JSON shape a
// calculator returns and lays it out as sheets, with no per-calculator
// code required when a new calculator is added.
package xlsx

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Object is a JSON object that preserves key order — encoding/json's
// map[string]any does not, and order matters here: it's what keeps a
// schedule sheet's columns in the same n/dueDate/amount/.../balance
// order the API (and every client's table) already uses.
type Object struct {
	Keys   []string
	Values map[string]any
}

func newObject() *Object {
	return &Object{Values: map[string]any{}}
}

func (o *Object) set(key string, v any) {
	if _, exists := o.Values[key]; !exists {
		o.Keys = append(o.Keys, key)
	}
	o.Values[key] = v
}

// ParseObject decodes a JSON object preserving field order. Values in
// the returned tree are: string, json.Number, bool, nil, *Object,
// []any (elements are any of the same set, so an array of objects is
// []any of *Object).
func ParseObject(raw json.RawMessage) (*Object, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, fmt.Errorf("expected a JSON object, got %v", tok)
	}
	obj, err := decodeObjectBody(dec)
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func decodeObjectBody(dec *json.Decoder) (*Object, error) {
	obj := newObject()
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, _ := keyTok.(string)

		val, err := decodeValue(dec)
		if err != nil {
			return nil, err
		}
		obj.set(key, val)
	}
	// consume the closing '}'
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	return obj, nil
}

func decodeValue(dec *json.Decoder) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			return decodeObjectBody(dec)
		case '[':
			var arr []any
			for dec.More() {
				v, err := decodeValue(dec)
				if err != nil {
					return nil, err
				}
				arr = append(arr, v)
			}
			if _, err := dec.Token(); err != nil { // ']'
				return nil, err
			}
			return arr, nil
		}
		return nil, fmt.Errorf("unexpected delimiter %v", t)
	default:
		return t, nil // string, json.Number, bool, or nil
	}
}
