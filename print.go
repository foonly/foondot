package main

import (
	"fmt"
	"os"
)

/**
 * Prints a formatted message to the console.
 * If three or more strings are provided, prints in the format: "<prefix>: <value> => <result>".
 * If two strings are provided, prints in the format: "<prefix>: <value>".
 * If one string is provided, prints just that string.
 * If the global 'color' variable is true, applies color formatting to the output.
 */
func printMessage(text ...string) {
	if len(text) >= 3 {
		if color {
			fmt.Fprintf(os.Stdout, "%s: %s%s%s => %s%s%s\n", text[0], colorGreen, text[1], colorNone, colorYellow, text[2], colorNone)
		} else {
			fmt.Fprintf(os.Stdout, "%s: %s => %s\n", text[0], text[1], text[2])
		}
	} else if len(text) == 2 {
		if color {
			fmt.Fprintf(os.Stdout, "%s: %s%s%s\n", text[0], colorYellow, text[1], colorNone)
		} else {
			fmt.Fprintf(os.Stdout, "%s: %s\n", text[0], text[1])
		}
	} else {
		fmt.Fprintf(os.Stdout, "%s\n", text[0])
	}
}

/**
 * Prints a formatted error message to the standard error stream.
 * If three or more strings are provided, prints in the format: "<prefix>: <value>\n<error message>".
 * If two strings are provided, prints in the format: "<prefix>: <value>".
 * If one string is provided, prints just that string.
 * If the global 'color' variable is true, applies color formatting to the output.
 */
func printError(text ...string) {
	if len(text) >= 3 {
		if color {
			fmt.Fprintf(os.Stderr, "%s%s: %s%s%s\n%s\n", colorRed, text[0], colorYellow, text[1], colorNone, text[2])
		} else {
			fmt.Fprintf(os.Stderr, "%s: %s\n%s\n", text[0], text[1], text[2])
		}
	} else if len(text) == 2 {
		if color {
			fmt.Fprintf(os.Stderr, "%s%s: %s%s%s\n", colorRed, text[0], colorYellow, text[1], colorNone)
		} else {
			fmt.Fprintf(os.Stderr, "%s: %s\n", text[0], text[1])
		}
	} else {
		if color {
			fmt.Fprintf(os.Stderr, "%s%s\n", colorRed, text[0])
		} else {
			fmt.Fprintf(os.Stderr, "%s\n", text[0])
		}
	}
}
