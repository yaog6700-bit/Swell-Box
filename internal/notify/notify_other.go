//go:build !windows

package notify

import "log"

func show(title, message string, isError bool) {
	if isError {
		log.Printf("[notify:error] %s: %s", title, message)
		return
	}
	log.Printf("[notify] %s: %s", title, message)
}
