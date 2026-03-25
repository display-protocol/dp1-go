package jcs

import (
	"strings"
	"testing"
)

func TestTransform_ObjectKeyOrder(t *testing.T) {
	t.Parallel()
	in := []byte(`{"b":1,"a":2}`)
	out, err := Transform(in)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `{"a":2,"b":1}` {
		t.Fatalf("unexpected canonical form: %s", out)
	}
}

func TestTransform_okCases(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty_object", `{}`, `{}`},
		{"empty_array", `[]`, `[]`},
		{"array_preserves_order", `[3,1,2]`, `[3,1,2]`},
		{"lexicographic_keys", `{"z":1,"aa":2}`, `{"aa":2,"z":1}`},
		{"nested_sorts_keys", `{"outer":{"b":1,"a":2}}`, `{"outer":{"a":2,"b":1}}`},
		{"null_in_object", `{"v":null}`, `{"v":null}`},
		{"true_in_object", `{"v":true}`, `{"v":true}`},
		{"false_in_object", `{"v":false}`, `{"v":false}`},
		{"string", `"hello"`, `"hello"`},
		{"string_unicode", `"\u0041"`, `"A"`},
		{"object_with_null", `{"x":null}`, `{"x":null}`},
		{"integer", `{"n":42}`, `{"n":42}`},
		{"mixed_types", `{"arr":[1,"two",true,null],"obj":{"z":0}}`, `{"arr":[1,"two",true,null],"obj":{"z":0}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out, err := Transform([]byte(tc.in))
			if err != nil {
				t.Fatal(err)
			}
			if string(out) != tc.want {
				t.Fatalf("got %q want %q", out, tc.want)
			}
		})
	}
}

func TestTransform_invalidJSON(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
	}{
		{"truncated", `{"a":`},
		{"bare_word", `undefined`},
		{"trailing_comma", `{"a":1,}`},
		// gowebpki/jcs expects a composite JSON text; bare scalars error at EOF.
		{"bare_null", `null`},
		{"bare_true", `true`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := Transform([]byte(tc.in))
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestTransform_idempotent(t *testing.T) {
	t.Parallel()
	in := []byte(`{"b":[{"y":2,"x":1}],"a":0}`)
	first, err := Transform(in)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Transform(first)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("second pass changed output:\nfirst:  %s\nsecond: %s", first, second)
	}
}

func TestTransform_whitespace_stripped(t *testing.T) {
	t.Parallel()
	in := []byte("{\n  \"b\" : 2 ,\n  \"a\" : 1\n}")
	out, err := Transform(in)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `{"a":1,"b":2}` {
		t.Fatalf("got %q", out)
	}
}

func TestTransform_deepNesting(t *testing.T) {
	t.Parallel()
	// 50 levels: each object has one key "k" wrapping the next.
	var b strings.Builder
	b.WriteString(`{"k":`)
	for range 49 {
		b.WriteString(`{"k":`)
	}
	b.WriteString(`0`)
	for range 50 {
		b.WriteString(`}`)
	}
	in := b.String()
	out, err := Transform([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	out2, err := Transform(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(out2) {
		t.Fatal("not idempotent on deep tree")
	}
	if !strings.HasPrefix(string(out), `{"k":`) {
		prefixLen := min(len(out), 20)
		t.Fatalf("unexpected prefix: %s", out[:prefixLen])
	}
}
