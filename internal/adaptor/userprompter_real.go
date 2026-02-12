// SPDX-License-Identifier: Apache-2.0
package adaptor

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// RealUserPrompter implements port.UserPrompter using stdin/stdout.
type RealUserPrompter struct{}

func (p *RealUserPrompter) Confirm(msg string) (bool, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Fprintf(os.Stderr, "%s [y/N]: ", msg)
	text, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	text = strings.TrimSpace(strings.ToLower(text))
	return text == "y" || text == "yes", nil
}

func (p *RealUserPrompter) Prompt(msg string, defaultVal string) (string, error) {
	reader := bufio.NewReader(os.Stdin)
	if defaultVal != "" {
		fmt.Fprintf(os.Stderr, "%s [%s]: ", msg, defaultVal)
	} else {
		fmt.Fprintf(os.Stderr, "%s: ", msg)
	}
	text, err := reader.ReadString('\n')
	if err != nil {
		return defaultVal, err
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return defaultVal, nil
	}
	return text, nil
}
