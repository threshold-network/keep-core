package clientinfo

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Gauge is a metric type that represents a single numerical value that can
// arbitrarily go up and down.
type Gauge struct {
	name   string
	labels map[string]string

	value     float64
	timestamp int64 // timestamp expressed as milliseconds
	mutex     sync.RWMutex
}

// Set allows setting the gauge to an arbitrary value.
func (g *Gauge) Set(value float64) {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	g.value = value
	g.timestamp = time.Now().UnixNano() / 1e6
}

// Exposes the gauge in the text-based exposition format.
func (g *Gauge) expose() string {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	typeLine := fmt.Sprintf("# TYPE %v %v", g.name, "gauge")

	labelsStrings := make([]string, 0)
	for name, value := range g.labels {
		labelsStrings = append(
			labelsStrings,
			fmt.Sprintf("%v=\"%v\"", name, value),
		)
	}
	labels := strings.Join(labelsStrings, ",")

	if len(labels) > 0 {
		labels = "{" + labels + "}"
	}

	body := fmt.Sprintf("%v%v %v %v", g.name, labels, g.value, g.timestamp)

	return fmt.Sprintf("%v\n%v", typeLine, body)
}
