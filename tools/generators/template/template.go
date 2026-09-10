package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/keep-network/keep-core/pkg/generate"
)

const (
	expectedArgs         = 3
	templateFileArgIndex = 1
	goFileArgIndex       = 2
	goExtension          = ".go"
)

// The template generator takes a file and lifts its contents into a Go string
// variable in a named Go file in package main.
//
// <executable> <template-file> <go-file>
//
// The assumption is made that the string is already UTF-8.
func main() {
	if len(os.Args) != expectedArgs {
		errorAndExit(fmt.Sprintf("Need exactly %d arguments.", expectedArgs))
	}

	templateFile := os.Args[templateFileArgIndex]

	// #nosec G304 (file path provided as taint input)
	// This line is placed in the auxiliary generator code,
	// not in the core application. User input has to be passed to provide a
	// path to the template file.
	templateContents, err := ioutil.ReadFile(templateFile)
	if err != nil {
		errorAndExit(fmt.Sprintf("Failed to open template file: [%v].", err))
	}

	goFilePath := os.Args[goFileArgIndex]
	if goFilePath[len(goFilePath)-len(goExtension):] != goExtension {
		errorAndExit("Go file should end in .go.")
	}
	goVariable := pathToVariable(goFilePath)

	goFileContents :=
		bytes.NewBufferString(fmt.Sprintf(
			"package main\n\n"+
				"// %s contains the template string from %s\n"+
				"var %s = `%s`\n",
			goVariable,
			filepath.Base(templateFile),
			goVariable,
			strings.ReplaceAll(string(templateContents), "`", "` + \"`\" + `"),
		))

	err = generate.OrganizeImports(goFileContents, goFilePath)
	if err != nil {
		errorAndExit(fmt.Sprintf(
			"Failed to produce a compiling Go file: [%v]; base code was:\n%s\n",
			err,
			goFileContents,
		))
	}

	err = generate.SaveBufferToFile(goFileContents, goFilePath)
	if err != nil {
		errorAndExit(fmt.Sprintf(
			"Failed to write generated Go file to %v: [%v].",
			goFilePath,
			err,
		))
	}
}

func errorAndExit(err string) {
	fmt.Fprintf(os.Stderr, "%s\n\n", err)
	fmt.Println(helpText(os.Args[0]))

	os.Exit(1)
}

func helpText(programName string) string {
	return programName + " <template-file> <go-file.go>\n" +
		"\tGenerates a Go file named <go-file> that consists of a single Go\n" +
		"\tvariable declaration containing the string contents of <template-file>.\n" +
		"\tThe Go variable is the camel-cased version of <go-file> and is placed\n" +
		"\tin package main.\n"
}

func pathToVariable(filePath string) string {
	base := filepath.Base(filePath)
	withoutExtension := base[0 : len(base)-len(goExtension)]
	splitAtUnderscores := strings.Split(withoutExtension, "_")

	// Uppercase all but the first underscored chunk.
	for i, chunk := range splitAtUnderscores[1:] {
		splitAtUnderscores[i+1] = strings.ToUpper(string(chunk[0])) + chunk[1:]
	}

	// Join everything into one camel-cased variable name.
	return strings.Join(splitAtUnderscores, "")
}
