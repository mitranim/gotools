package render

// Public render functions.

import (
	"html/template"
	"strings"
)

/**
 * Shorthand rendering function. Renders the page at the given path,
 * automatically falling back to error pages corresponding to the kinds of
 * errors that may occur (404, 500, possibly others). Returns the rendered
 * bytes and the last error that occurred in the process.
 *
 * The caller routine should always examine the error with ErrorCode(err) to
 * retrieve the http status code to set in the response handler.
 *
 * Note that rendering is always going to be successful; the role of the error
 * is not to signal a complete failure, but to carry the information about the
 * character of the problem (if any) that occurred in the process.
 *
 * Also see the RenderError comment.
 */
func Render(path string, data map[string]interface{}) ([]byte, error) {
	assertReady()

	bytes, err := RenderPage(path, data)

	if err != nil {
		return RenderError(err, data)
	}

	return bytes, nil
}

// Takes a path to a page and a data map. Renders the page and, hierarchically,
// all layouts enclosing it, up to the root, passing the data map to each
// template.
func RenderPage(path string, data map[string]interface{}) ([]byte, error) {
	assertReady()

	// Check for nil map.
	if data == nil {
		data = map[string]interface{}{}
	}

	// Validate and adjust path.
	path, err := parsePath(path, Pages)
	if err != nil {
		return nil, err
	}

	// Build an array of nested template paths.
	paths := pathsToTemplates(path)

	// Render the template into each enclosing layout.
	for _, pt := range paths {
		bytes, err := renderAt(pt, data, Pages)
		if err != nil {
			return nil, err
		}
		// Enclose the content.
		data["content"] = template.HTML(strings.TrimSpace(string(bytes)))
	}

	// Grab the resulting content as bytes.
	html, _ := data["content"].(template.HTML)

	return []byte(html), nil
}

// Renders a standalone template at the given path. Unlike pages, names of
// standalones may begin with $.
func RenderStandalone(path string, data map[string]interface{}) ([]byte, error) {
	assertReady()

	// A template must exist.
	if Standalone.Lookup(path) == nil {
		return nil, err404
	}

	return renderAt(path, data, Standalone)
}

/**
 * Takes an error and renders a page at the path corresponding to the error,
 * automatically falling back to other error pages if a different error occurs.
 * Returns the rendered bytes and the last error that occurred in the process.
 *
 * Error codes are translated into template paths by using either the CodePath
 * function provided during setup, or a simple integer-to-string conversion
 * (default). If your error pages are located in the root of the pages folder
 * and have names like "404.html" or "500.gohtml", they will be used
 * automatically.
 *
 * The caller routine should always examine the error with ErrorCode(err) to
 * retrieve the http status code to set in the response handler.
 *
 * Note that rendering is always going to be successful; the role of the error
 * is not to signal a complete failure, but to carry the information about the
 * character of the problem (if any) that occurred in the process.
 */
func RenderError(err error, data map[string]interface{}) (bytes []byte, lastErr error) {
	// Map of error codes that have occurred at least once.
	codes := map[int]bool{}

	/**
	 * Algorithm:
	 *  * attempt to render each non-500 error once; if the same code repeats,
	 *  	fall through to 500
	 *  * attempt to render 500 once; if 500 repeats, fall back on bytes
	 *  * if something is rendered without an error, immediately break and return
	 *  	the result plus the last non-nil error
	 */

	for err != nil {
		lastErr = err
		code := ErrorCode(err)

		// Repeated error code.
		if codes[code] {
			// Double 500 -> fall back on bytes.
			if code == 500 {
				log("internal rendering error")
				// Use the provided UltimateFailure data, if possible.
				if len(conf.UltimateFailure) > 0 {
					bytes = conf.UltimateFailure
					// Otherwise use the default message.
				} else {
					bytes = []byte(err500ISE)
				}
				break
			}
			// Non-500 -> convert to 500 to try to render the 500 page.
			code = 500
		}

		// Register the code.
		codes[code] = true

		// Try to render the matching page.
		bytes, err = RenderPage(ErrorPath(err), data)
	}

	return
}
