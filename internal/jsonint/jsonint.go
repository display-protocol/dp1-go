// Package jsonint decodes JSON Schema "integer" members into Go ints.
//
// JSON Schema's integer is a property of the value, not of its spelling: 100, 100.0 and
// 1e2 are all valid, and the SDK validates before it decodes. encoding/json accepts only
// the first into an int, so a schema-valid document failed one step after validation.
package jsonint

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
)

// Int is an int that decodes from any JSON number with an integral value. Types with an
// integer member convert their *int field to *Int for the duration of a decode, so the
// exported field type stays *int.
type Int int

// UnmarshalJSON rejects what the schema's integer rejects (strings, fractions, other JSON
// types) and values outside Go's int, which are never truncated.
//
// The error is a *json.UnmarshalTypeError, as for a plain int field, so errors.As keeps
// working and encoding/json fills in the member name.
func (i *Int) UnmarshalJSON(b []byte) error {
	lit := string(b)
	if lit == "null" {
		return nil
	}
	n, ok := parse(lit)
	if !ok {
		return &json.UnmarshalTypeError{Value: describe(lit), Type: reflect.TypeOf(0)}
	}
	*i = Int(n)
	return nil
}

func describe(lit string) string {
	switch lit[0] {
	case '"':
		return "string"
	case '{':
		return "object"
	case '[':
		return "array"
	case 't', 'f':
		return "bool"
	}
	return "number " + lit
}

// parse reads a JSON value as an exact integer. It works on the digits rather than going
// through float64 (which rounds above 2^53) or big.Rat (which expands 1e1000000 in full
// before anything can refuse it): the work here is bounded by the literal's own length.
func parse(lit string) (int, bool) {
	if lit == "" || (lit[0] != '-' && (lit[0] < '0' || lit[0] > '9')) {
		return 0, false
	}
	sign := ""
	if lit[0] == '-' {
		sign, lit = "-", lit[1:]
	}
	mantissa, exponent := lit, ""
	if at := strings.IndexAny(lit, "eE"); at >= 0 {
		mantissa, exponent = lit[:at], lit[at+1:]
	}
	whole, frac, _ := strings.Cut(mantissa, ".")
	digits := strings.TrimLeft(whole+frac, "0")
	if digits == "" {
		return 0, true // zero under any exponent
	}
	exp := 0
	if exponent != "" {
		var err error
		if exp, err = strconv.Atoi(exponent); err != nil {
			return 0, false // non-zero value under an exponent wider than int
		}
	}
	// No int has more than 19 digits, and no digit string absorbs a negative exponent
	// longer than itself. Checked before any arithmetic on exp, which came off the wire
	// and could wrap.
	if exp > len(lit)+19 || exp < -len(lit) {
		return 0, false
	}
	significant := strings.TrimRight(digits, "0")
	// scale is the power of ten applied to significant; negative means a fraction.
	scale := exp - len(frac) + len(digits) - len(significant)
	if scale < 0 || len(significant)+scale > 19 {
		return 0, false
	}
	n, err := strconv.Atoi(sign + significant + strings.Repeat("0", scale))
	return n, err == nil
}
