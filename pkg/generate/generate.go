// Package generate contains helper functions for code generators.
package generate

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/tools/imports"
)

// OrganizeImports takes a buffer containing Go code and a file path where that
// code will live, resolves the imports and formats the code, and writes the
// resulting formatted code into the buffer.
//
// Returns nil if the code was processed and the buffer is now updated with the
// organized, formatted code, or an error if something went wrong during the
// import+format process.
func OrganizeImports(codeBuffer *bytes.Buffer, filePath string) error {
	// Resolve imports
	code, err := imports.Process(filePath, codeBuffer.Bytes(), nil)
	if err != nil {
		return fmt.Errorf("failed to find/resolve imports [%v]", err)
	}

	// Write organized code to the buffer.
	codeBuffer.Reset()
	if _, err := codeBuffer.Write(code); err != nil {
		return fmt.Errorf("cannot write code to buffer [%v]", err)
	}

	return nil
}

// SaveBufferToFile stores the given buffer's contents to a file at `filePath`.
//
// Returns nil if the file was written successfully, or an error if there was an
// error writing the file.
func SaveBufferToFile(buffer *bytes.Buffer, filePath string) error {
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

	if _, err := buffer.WriteTo(file); err != nil {
		return fmt.Errorf("writing to output file %s failed [%v]", filePath, err)
	}

	return nil
}
