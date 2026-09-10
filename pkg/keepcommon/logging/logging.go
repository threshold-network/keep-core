package logging

import (
	"fmt"
	"strings"

	logging "github.com/ipfs/go-log"
)

// Configure sets up logging configuration from the given string. This string is
// expected to have a space-delimited set of level directives, which are each
// evaluated in order by the evaluateLevelDirective function.
//
// If the level directive string is empty, a default level of info for all keep*
// subsystems is imposed.
func Configure(levelDirectiveString string) error {
	// Default to info logs for keep.
	if len(levelDirectiveString) == 0 {
		levelDirectiveString = "keep*=info"
	}

	levelDirectives := strings.Split(levelDirectiveString, " ")
	for _, directive := range levelDirectives {
		err := evaluateLevelDirective(directive)
		if err != nil {
			return fmt.Errorf(
				"Failed to parse log level directive [%s]: [%v]\n"+
					"Directives can be any of:\n"+
					" - a global log level, e.g. 'debug'\n"+
					" - a subsystem=level pair, e.g. 'keep-relay=info'\n"+
					" - a subsystem*=level prefix pair, e.g. 'keep*=warn'\n",
				directive,
				err,
			)
		}
	}

	return nil
}

// Takes a levelDirective that can have one of three formats:
//
//     <log-level> |
//     <subsystem>=<log-level> |
//     <subsystem-prefix>*=<log-level>
//
// In the first form, the given log-level is set on all subsystems.
//
// In the second form, the given log-level is set on the given subsystem.
//
// In the third form, the given log-level is set on any subsystem that starts
// with the given subsystem-prefix.
//
// Supported log levels are as per the ipfs/go-logging library.
func evaluateLevelDirective(levelDirective string) error {
	splitLevel := strings.Split(levelDirective, "=")

	switch len(splitLevel) {
	case 1:
		level := splitLevel[0]

		err := logging.SetLogLevel("*", level)
		if err != nil {
			return err
		}

	case 2:
		levelSubsystem := splitLevel[0]
		level := splitLevel[1]

		if strings.HasSuffix(levelSubsystem, "*") {
			subsystemPrefix := strings.TrimSuffix(levelSubsystem, "*")
			// Wildcard suffix, check for matching subsystems.
			for _, subsystem := range logging.GetSubsystems() {
				if strings.HasPrefix(subsystem, subsystemPrefix) {
					err := logging.SetLogLevel(subsystem, level)
					if err != nil {
						return err
					}
				}
			}
		} else {
			return logging.SetLogLevel(levelSubsystem, level)
		}

	default:
		return fmt.Errorf("more than two =-delimited components in directive")
	}

	return nil
}
