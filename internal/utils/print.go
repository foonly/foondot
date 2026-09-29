package utils

import (
	"fmt"
	"os"
)

// Color enables colored output. It is a process-wide output setting, set once
// from the command line or config before anything is printed.
var Color = false

const (
	colorNone   = "\033[0m"
	colorRed    = "\033[0;31m"
	colorGreen  = "\033[0;32m"
	colorYellow = "\033[0;33m"
)

// PrintMessage prints a plain message to standard output.
func PrintMessage(text string) {
	fmt.Fprintf(os.Stdout, "%s\n", text)
}

// PrintValue prints a message with a highlighted value to standard output,
// in the format "<label>: <value>".
func PrintValue(label, value string) {
	if Color {
		fmt.Fprintf(os.Stdout, "%s: %s%s%s\n", label, colorYellow, value, colorNone)
	} else {
		fmt.Fprintf(os.Stdout, "%s: %s\n", label, value)
	}
}

// PrintChange prints a message about something changing from one value to
// another to standard output, in the format "<label>: <from> => <to>".
func PrintChange(label, from, to string) {
	if Color {
		fmt.Fprintf(os.Stdout, "%s: %s%s%s => %s%s%s\n", label, colorGreen, from, colorNone, colorYellow, to, colorNone)
	} else {
		fmt.Fprintf(os.Stdout, "%s: %s => %s\n", label, from, to)
	}
}

// PrintWarning prints a message without a value to standard error.
func PrintWarning(text string) {
	if Color {
		fmt.Fprintf(os.Stderr, "%s%s%s\n", colorRed, text, colorNone)
	} else {
		fmt.Fprintf(os.Stderr, "%s\n", text)
	}
}

// PrintError prints an error message with a highlighted value to standard
// error, in the format "<label>: <value>".
func PrintError(label, value string) {
	if Color {
		fmt.Fprintf(os.Stderr, "%s%s: %s%s%s\n", colorRed, label, colorYellow, value, colorNone)
	} else {
		fmt.Fprintf(os.Stderr, "%s: %s\n", label, value)
	}
}

// PrintErrorCause prints an error message with a highlighted value and the
// underlying error on the next line to standard error.
func PrintErrorCause(label, value string, cause error) {
	if Color {
		fmt.Fprintf(os.Stderr, "%s%s: %s%s%s\n%s\n", colorRed, label, colorYellow, value, colorNone, cause)
	} else {
		fmt.Fprintf(os.Stderr, "%s: %s\n%s\n", label, value, cause)
	}
}
