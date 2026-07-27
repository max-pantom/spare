package doctor

import (
	"fmt"
	"strconv"
)

func integerFormat(value int64) string {
	return strconv.FormatInt(value, 10)
}

func oneDecimal(value float64) string {
	return fmt.Sprintf("%.1f", value)
}
