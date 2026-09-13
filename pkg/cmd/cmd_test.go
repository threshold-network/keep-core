package cmd

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/spf13/cobra"
)

func TestComposableArgCheckerComposesOnSuccess(t *testing.T) {
	var seenFunctions []int

	firstComposer := ComposableArgChecker(func(*cobra.Command, []string) error {
		seenFunctions = append(seenFunctions, 1)
		return nil
	})
	secondComposer := ComposableArgChecker(func(*cobra.Command, []string) error {
		seenFunctions = append(seenFunctions, 2)
		return nil
	})
	thirdComposer := ComposableArgChecker(func(*cobra.Command, []string) error {
		seenFunctions = append(seenFunctions, 3)
		return nil
	})

	tests := map[string]struct {
		composed ComposableArgChecker
		sequence []int
	}{
		"1-2-3": {
			composed: firstComposer.AndThen(secondComposer).AndThen(thirdComposer),
			sequence: []int{1, 2, 3},
		},
		"3-2-1": {
			composed: thirdComposer.AndThen(secondComposer).AndThen(firstComposer),
			sequence: []int{3, 2, 1},
		},
		"2-1-3": {
			composed: secondComposer.AndThen(firstComposer).AndThen(thirdComposer),
			sequence: []int{2, 1, 3},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			seenFunctions = []int{}
			result := test.composed(nil, []string{})

			if !reflect.DeepEqual(seenFunctions, test.sequence) {
				t.Errorf(
					"\nexpected: [%v]\nactual:   [%v]",
					test.sequence,
					seenFunctions,
				)
			}

			if result != nil {
				t.Errorf(
					"\nexpected: [%v]\nactual:   [%v]",
					nil,
					result,
				)
			}
		})
	}
}

func TestComposableArgCheckerShortCircuitsOnFailure(t *testing.T) {
	var seenFunctions []int

	firstComposer := ComposableArgChecker(func(*cobra.Command, []string) error {
		seenFunctions = append(seenFunctions, 1)
		return nil
	})
	secondComposer := ComposableArgChecker(func(*cobra.Command, []string) error {
		seenFunctions = append(seenFunctions, 2)
		return nil
	})
	thirdComposer := ComposableArgChecker(func(*cobra.Command, []string) error {
		seenFunctions = append(seenFunctions, 3)
		return nil
	})
	errorComposer := ComposableArgChecker(func(*cobra.Command, []string) error {
		return fmt.Errorf("ohai")
	})

	tests := map[string]struct {
		composed     ComposableArgChecker
		sequence     []int
		errorMessage string
	}{
		"1-2-error-3": {
			composed:     firstComposer.AndThen(secondComposer).AndThen(errorComposer).AndThen(thirdComposer),
			sequence:     []int{1, 2},
			errorMessage: "ohai",
		},
		"3-error-2-1": {
			composed:     thirdComposer.AndThen(errorComposer).AndThen(secondComposer).AndThen(firstComposer),
			sequence:     []int{3},
			errorMessage: "ohai",
		},
		"2-1-error-3": {
			composed:     secondComposer.AndThen(firstComposer).AndThen(errorComposer).AndThen(thirdComposer),
			sequence:     []int{2, 1},
			errorMessage: "ohai",
		},
		"3-1-2-error": {
			composed:     thirdComposer.AndThen(firstComposer).AndThen(secondComposer).AndThen(errorComposer),
			sequence:     []int{3, 1, 2},
			errorMessage: "ohai",
		},
		"3-1-2": {
			composed:     thirdComposer.AndThen(firstComposer).AndThen(secondComposer),
			sequence:     []int{3, 1, 2},
			errorMessage: "",
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			seenFunctions = []int{}
			err := test.composed(nil, []string{})

			if !reflect.DeepEqual(seenFunctions, test.sequence) {
				t.Errorf(
					"\nexpected: [%v]\nactual:   [%v]",
					test.sequence,
					seenFunctions,
				)
			}

			message := ""
			if err != nil {
				message = err.Error()
			}

			if message != test.errorMessage {
				t.Errorf(
					"\nexpected: [%v]\nactual:   [%v]",
					test.errorMessage,
					message,
				)
			}
		})
	}
}
