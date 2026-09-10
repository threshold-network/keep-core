//go:generate go run github.com/keep-network/keep-core/tools/generators/template contract_const_methods.go.tmpl contract_const_methods_template_content.go
//go:generate go run github.com/keep-network/keep-core/tools/generators/template contract_non_const_methods.go.tmpl contract_non_const_methods_template_content.go
//go:generate go run github.com/keep-network/keep-core/tools/generators/template contract_events.go.tmpl contract_events_template_content.go
//go:generate go run github.com/keep-network/keep-core/tools/generators/template contract.go.tmpl contract_template_content.go
//go:generate go run github.com/keep-network/keep-core/tools/generators/template command.go.tmpl command_template_content.go

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"golang.org/x/tools/imports"
)

// Main function. Expects to be invoked as:
//
//	<executable> <input.abi> contract/<contract_output.go> cmd/<cmd_output.go>
//
// The first file will receive a contract binding that is slightly higher-level
// than abigen's output, including an event-based interface for contract event
// interaction, support for revert error reporting, serialized transaction
// submission, and simplified transactor handling.
//
// The second file will receive an spf13/cobra-compatible Command object
// that can be used to add command-line interaction with the specified contract
// by adding the relevant commands to a top-level cobra.Command object. The
// file's initializer will currently append the command object to an exported
// package variable named Command in the same package that the command
// itself is in. This variable is NOT generated; instead, it is expected that it
// will be set up out-of-band in the package.
//
// Note that currently the packages for contract and command are hardcoded to
// contract and cmd, respectively.
func main() {
	hostChainModule := flag.String(
		"host-chain-module",
		"github.com/ethereum/go-ethereum",
		"Ethereum host chain Go module imported from the generated code",
	)

	chainUtilPackage := flag.String(
		"chain-util-package",
		"github.com/keep-network/keep-core/pkg/chain/ethereumutil/ethutil",
		"Host chain utils package imported from the generated code",
	)

	genPackage := flag.String(
		"gen-package",
		"",
		"Import path of the gen/ directory bindings are generated into; used "+
			"to explicitly import the sibling abi (and contract) packages "+
			"since this module has several like-named gen/abi packages that "+
			"goimports cannot reliably disambiguate. Optional: leave empty "+
			"to fall back to goimports auto-resolution.",
	)

	flag.Parse()

	// Two leading arguments (`input.abi` and `contract_output.go`) are required.
	// The third argument (`cmd_output.go`) is optional.
	if !(flag.NArg() == 2 || flag.NArg() == 3) {
		panic(fmt.Sprintf(
			"Expected `%v <input.abi> <contract_output.go> [cmd_output.go]`, but got [%v].",
			os.Args[0],
			os.Args,
		))
	}

	abiPath := flag.Arg(0)
	contractOutputPath := flag.Arg(1)
	commandOutputPath := flag.Arg(2)

	// #nosec G304 (file path provided as taint input)
	// This line is placed in the auxiliary generator code,
	// not in the core application. User input has to be passed to
	// provide a path to the contract ABI.
	abiFile, err := ioutil.ReadFile(abiPath)
	if err != nil {
		panic(fmt.Sprintf(
			"Failed to read ABI file at [%v]: [%v].",
			abiPath,
			err,
		))
	}

	templates, err := parseTemplates()
	if err != nil {
		panic(fmt.Sprintf("Failed to parse templates: [%v].", err))
	}

	abi, err := abi.JSON(strings.NewReader(string(abiFile)))
	if err != nil {
		panic(fmt.Sprintf(
			"Failed to parse ABI at [%v]: [%v].",
			abiPath,
			err,
		))
	}

	var payableInfo []methodPayableInfo
	err = json.Unmarshal(abiFile, &payableInfo)
	if err != nil {
		panic(fmt.Sprintf(
			"Failed to parse additional ABI metadata at [%v]: [%v].",
			abiPath,
			err,
		))
	}

	// The name of the ABI binding Go class is the same as the filename of the
	// ABI file, minus the extension.
	abiClassName := path.Base(abiPath)
	abiClassName = abiClassName[0 : len(abiClassName)-4] // strip .abi
	contractInfo := buildContractInfo(
		*hostChainModule,
		*chainUtilPackage,
		*genPackage,
		abiClassName,
		&abi,
		payableInfo,
	)

	contractBuf, err := generateCode(
		contractOutputPath,
		templates,
		"contract.go.tmpl",
		&contractInfo,
	)
	if err != nil {
		panic(fmt.Sprintf(
			"Failed to generate Go file for contract [%v] at [%v]: [%v].",
			contractInfo.AbiClass,
			contractOutputPath,
			err,
		))
	}

	// Save the contract code to a file. We save the code before running command
	// code generation as the command code imports bits of contract code and we
	// need to resolve these imports on command code imports organization.
	if err := saveBufferToFile(contractBuf, contractOutputPath); err != nil {
		panic(fmt.Sprintf(
			"Failed to save Go file at [%v]: [%v].",
			contractOutputPath,
			err,
		))
	}

	if len(commandOutputPath) > 0 {
		commandBuf, err := generateCode(
			commandOutputPath,
			templates,
			"command.go.tmpl",
			&contractInfo,
		)
		if err != nil {
			panic(fmt.Sprintf(
				"Failed to generate Go file at [%v]: [%v].",
				commandOutputPath,
				err,
			))
		}

		// Save the command code to a file.
		if err := saveBufferToFile(commandBuf, commandOutputPath); err != nil {
			panic(fmt.Sprintf(
				"Failed to save Go file at [%v]: [%v].",
				commandOutputPath,
				err,
			))
		}
	}
}

func parseTemplates() (*template.Template, error) {
	templates := map[string]string{
		"contract_const_methods.go.tmpl":     contractConstMethodsTemplateContent,
		"contract_non_const_methods.go.tmpl": contractNonConstMethodsTemplateContent,
		"contract_events.go.tmpl":            contractEventsTemplateContent,
		"contract.go.tmpl":                   contractTemplateContent,
		"command.go.tmpl":                    commandTemplateContent,
	}

	combinedTemplate := template.New("")
	for name, content := range templates {
		var err error
		// FIXME The generator should probably emit the {{define}}/{{end}}
		// FIXME blocks itself.
		combinedTemplate, err = combinedTemplate.Parse("{{define \"" + name + "\"}}" + content + "{{end}}")
		if err != nil {
			return nil, fmt.Errorf("failed to parse template %s: %w", name, err)
		}
	}
	return combinedTemplate, nil
}

// Generates code by applying the named template in the passed template bundle
// to the specified data object. Writes the output to a buffer and then
// formats and organizes the imports on that buffer, returning the final result
// ready for emission onto the filesystem.
//
// Note that this means the generated file must compile, or import organization
// will fail. The error message in case of compilation failure will be bubbled
// up, but the file contents currently will not be written.
func generateCode(
	outFile string,
	templat *template.Template,
	templateName string,
	data interface{},
) (*bytes.Buffer, error) {
	var buffer bytes.Buffer

	if err := templat.ExecuteTemplate(&buffer, templateName, data); err != nil {
		return nil, fmt.Errorf(
			"generating code failed: [%v]",
			err,
		)
	}

	if err := organizeImports(outFile, &buffer); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return &buffer, nil
	}

	return &buffer, nil
}

// Resolves imports in a code stored in a Buffer.
func organizeImports(outFile string, buf *bytes.Buffer) error {
	// Resolve imports
	code, err := imports.Process(outFile, buf.Bytes(), nil)
	if err != nil {
		return fmt.Errorf("failed to find/resove imports [%v]", err)
	}

	// Write organized code to the buffer.
	buf.Reset()
	if _, err := buf.Write(code); err != nil {
		return fmt.Errorf("cannot write code to buffer [%v]", err)
	}

	return nil
}

// Stores the Buffer `buf` content to a file in `filePath`
func saveBufferToFile(buf *bytes.Buffer, filePath string) error {
	file, err := os.Create(filepath.Clean(filePath))

	// #nosec G104 G307 (audit errors not checked & deferring unsafe method)
	// This line is placed in the auxiliary generator code,
	// not in the core application. Also, the Close function returns only
	// the error. It doesn't return any other values which can be a security
	// threat when used without checking the error.
	defer file.Close()
	if err != nil {
		return fmt.Errorf("output file %s creation failed [%v]", filePath, err)
	}

	if _, err := buf.WriteTo(file); err != nil {
		return fmt.Errorf("writing to output file %s failed [%v]", filePath, err)
	}

	return nil
}
