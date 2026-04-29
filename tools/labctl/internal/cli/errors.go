package cli

import (
	"fmt"
	"io"
)

func renderError(w io.Writer, err error) {
	if err == nil {
		return
	}

	_, _ = fmt.Fprintf(w, "Error: %v\n", err)
}
