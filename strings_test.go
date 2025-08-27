package httptestclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_basic_map_can_be_expanded(t *testing.T) {
	keys := map[string]any{
		"lower": 1,
		"UPPER": "two",
		"Mixed": true,
		"aZ_09": -1,
	}
	actual := expandStr("before $lower$UPPER ($Mixed) $aZ_09 after $UNKNOWN", keys)

	assert.Equal(t, "before 1two (true) -1 after [no_key:$UNKNOWN]", actual)
}

func Test_when_no_expansions_are_supplied_expandStr_is_identity_function(t *testing.T) {
	assert.Equal(t, "no change", expandStr("no change"))
	assert.Equal(t, "", expandStr(""))
}

func Test_expansion_can_use_indexed_argument_values(t *testing.T) {
	a := someType{}
	actual := expandStr("$0, $1$2 @@$3@@ $99", 10, "any", true, a)

	assert.Equal(t, "10, anytrue @@has-stringer@@ [bad_index:$99]", actual)
}

type someType struct{}

func (someType) String() string { return "has-stringer" }
