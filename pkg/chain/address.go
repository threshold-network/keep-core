package chain

import (
	"fmt"
	"strings"
)

// Address is a chain-agnostic representation of a chain address.
type Address string

func (a Address) String() string {
	return string(a)
}

// Addresses is a list of Address.
type Addresses []Address

// Set transform Addresses into a set of unique items.
func (a Addresses) Set() map[Address]bool {
	set := make(map[Address]bool)

	for _, address := range a {
		set[address] = true
	}

	return set
}

// String converts Addresses into a string with elements indexed from 1.
// This allows to print the list of addresses as a group members with member
// indexes starting from 1.
func (a Addresses) String() string {
	if len(a) == 0 {
		return "[]"
	}

	if len(a) == 1 {
		return fmt.Sprintf("[1: %s]", a[0])
	}

	var sb strings.Builder
	var i int

	sb.WriteString("[")
	for i = 0; i < len(a)-1; i++ {
		fmt.Fprintf(&sb, "%d: %s, ", i+1, a[i].String())
	}
	fmt.Fprintf(&sb, "%d: %s]", i+1, a[i].String())

	return sb.String()
}
