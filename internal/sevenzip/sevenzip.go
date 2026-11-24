// Package sevenzip bundles the official Windows x86 7-Zip console program.
package sevenzip

import (
	"embed"
	"os"
	"path/filepath"
)

//go:embed windows-386/7z.exe windows-386/7z.dll
var binaries embed.FS

//go:embed windows-386/License.txt
var license string

//go:embed LGPL-2.1.txt
var lgpl string

const release = "https://github.com/ip7z/7zip/releases/download/26.06/"

// Notice returns the attribution, corresponding source URL and license texts.
func Notice() string {
	return "Bundled component: 7-Zip 26.06, Windows x86\n" +
		"Official distribution: " + release + "7z2606.exe\n" +
		"Corresponding source: " + release + "7z2606-src.tar.xz\n\n" +
		license + "\n" + lgpl
}

// Materialize writes the bundled CLI and its DLL to a new subdirectory of dir.
// The caller owns dir and should remove it once all 7-Zip processes exit.
func Materialize(dir string) (string, error) {
	binDir, err := os.MkdirTemp(dir, "7zip-")
	if err != nil {
		return "", err
	}
	for _, name := range []string{"7z.exe", "7z.dll"} {
		data, err := binaries.ReadFile("windows-386/" + name)
		if err == nil {
			err = os.WriteFile(filepath.Join(binDir, name), data, 0700)
		}
		if err != nil {
			os.RemoveAll(binDir)
			return "", err
		}
	}
	return filepath.Join(binDir, "7z.exe"), nil
}
