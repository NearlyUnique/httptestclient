package httptestclient

import (
	"fmt"
	"regexp"
	"strconv"
)

var rxDollarEnv = regexp.MustCompile(`\$(?P<Key>[a-zA-Z0-9_]+)`)

func expandStr(msg string, args ...any) string {
	return replaceAllStringSubMatchFunc(rxDollarEnv, msg, func(values []string) string {
		if i, err := strconv.Atoi(values[1]); err == nil {
			if i < 0 || i >= len(args) {
				return fmt.Sprintf("[bad_index:$%d]", i)
			}
			return fmt.Sprintf("%v", args[i])
		}
		if len(args) == 1 {
			if m, ok := args[0].(map[string]any); ok {
				if v, ok := m[values[1]]; ok {
					return fmt.Sprintf("%v", v)
				}
			}
		}
		return fmt.Sprintf("[no_key:$%s]", values[1])
	})
}

func replaceAllStringSubMatchFunc(re *regexp.Regexp, str string, repl func(args []string) string) string {
	result := ""
	lastIndex := 0

	for _, v := range re.FindAllSubmatchIndex([]byte(str), -1) {
		var groups []string
		for i := 0; i < len(v); i += 2 {
			groups = append(groups, str[v[i]:v[i+1]])
		}

		result += str[lastIndex:v[0]] + repl(groups)
		lastIndex = v[1]
	}

	return result + str[lastIndex:]
}
