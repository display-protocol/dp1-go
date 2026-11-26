package jsonint

import (
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
	"testing"
)

func TestInt_AcceptsEveryIntegralSpelling(t *testing.T) {
	t.Parallel()
	cases := map[string]int{
		`0`: 0, `-0`: 0, `100`: 100, `100.0`: 100, `1e2`: 100, `1E+2`: 100, `1.5e1`: 15,
		`1000e-1`: 100, `0.001e3`: 1, `-5.0`: -5, `0e99999999999999999999`: 0,
		`0.0e-99999999999999999999`:               0,
		"1" + strings.Repeat("0", 512) + "e-512":  1,
		"0." + strings.Repeat("0", 512) + "1e513": 1,
		strconv.Itoa(math.MaxInt):                 math.MaxInt,
		strconv.Itoa(math.MinInt):                 math.MinInt,
		strconv.Itoa(math.MaxInt) + ".0":          math.MaxInt,
	}
	for in, want := range cases {
		var got Int
		if err := json.Unmarshal([]byte(in), &got); err != nil {
			t.Errorf("%.40s: %v", in, err)
		} else if int(got) != want {
			t.Errorf("%.40s: got %d want %d", in, got, want)
		}
	}
}

// Nothing the schema's integer refuses may decode, and nothing outside int is truncated.
// The huge exponents also lock that no expansion happens before the refusal.
func TestInt_Rejects(t *testing.T) {
	t.Parallel()
	for _, in := range []string{
		`1.5`, `1e-1`, `100.0001`, `"100"`, `true`, `{}`, `[1]`,
		`1e19`, `9223372036854775808`, `-9223372036854775809`,
		`1e1000000000`, `1e-1000000000`, `1e9223372036854775807`, `1e-9223372036854775808`,
		`1e99999999999999999999`,
	} {
		var got Int
		err := json.Unmarshal([]byte(in), &got)
		var typeErr *json.UnmarshalTypeError
		if !errors.As(err, &typeErr) {
			t.Errorf("%s: got value %d, err %v; want *json.UnmarshalTypeError", in, got, err)
		}
	}
}
