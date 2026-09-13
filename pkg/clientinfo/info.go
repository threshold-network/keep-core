package clientinfo

import (
	"fmt"
	"strings"
)

// Info is a metric type that represents a constant information
// that cannot change in the time.
type Info struct {
	name   string
	labels map[string]string
}

// Exposes the info in the text-based exposition format.
func (i *Info) expose() string {
	labelsStrings := make([]string, 0)
	for name, value := range i.labels {
		labelsStrings = append(
			labelsStrings,
			fmt.Sprintf("%v=\"%v\"", name, value),
		)
	}
	labels := strings.Join(labelsStrings, ",")

	return fmt.Sprintf("%v{%v} %v", i.name, labels, "1")
}
