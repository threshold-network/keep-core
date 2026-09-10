package clientinfo

import "testing"

func TestInfoExpose(t *testing.T) {
	info := &Info{
		name:   "test_info",
		labels: map[string]string{"label": "value"},
	}

	actualText := info.expose()

	expectedText := "test_info{label=\"value\"} 1"

	if actualText != expectedText {
		t.Fatalf(
			"incorrect gauge expose text:\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedText,
			actualText,
		)
	}
}
