package recstation

import (
	"fmt"
	"strings"
)

func Tsprintf(format string, params map[string]interface{}) string {
	for key, val := range params {
		format = strings.Replace(format, ("{{" + key + "}}"), fmt.Sprintf("%v", val), -1)
	}

	return format
}
