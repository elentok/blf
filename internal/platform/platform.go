package platform

import (
	"github.com/atotto/clipboard"
	"github.com/pkg/browser"
)

func OpenURL(url string) error {
	return browser.OpenURL(url)
}

func CopyText(text string) error {
	return clipboard.WriteAll(text)
}
