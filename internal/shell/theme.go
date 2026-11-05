package shell

import (
	"golang.org/x/sys/windows/registry"
)

const personalizeKey = `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`

// systemIsDark reports whether the Windows apps theme is dark.
// AppsUseLightTheme is a DWORD where 0 = dark, 1 = light, absent = light.
func systemIsDark() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, personalizeKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()

	v, _, err := key.GetIntegerValue("AppsUseLightTheme")
	if err != nil {
		return false
	}
	return v == 0
}
