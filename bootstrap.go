package cleave

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/superp00t/etc"

	"github.com/superp00t/etc/yo"
)

func findGopath() etc.Path {
	if os.Getenv("GOPATH") != "" {
		return etc.ParseSystemPath(os.Getenv("GOPATH"))
	}

	if os.Getenv("HOME") != "" {
		return etc.ParseSystemPath(os.Getenv("HOME")).Concat("go")
	}

	if os.Getenv("USERPROFILE") != "" {
		return etc.ParseSystemPath(os.Getenv("USERPROFILE")).Concat("go")
	}

	yo.Fatal("Could not find Gopath.")
	return nil
}

func GetPlatformCompilers(platform string) (string, string, error) {
	str := strings.Split(platform, "-")
	if len(str) != 2 {
		return "", "", fmt.Errorf("malformed platform ID %s", platform)
	}

	return acquireCompiler(str[0], str[1])
}

func acquireCompiler(_os, arch string) (string, string, error) {
	if _os == runtime.GOOS && arch == runtime.GOARCH {
		gcc, err := exec.LookPath("gcc")
		if err != nil {
			return "", "", err
		}

		gpp, err := exec.LookPath("g++")
		if err != nil {
			return "", "", err
		}

		return gcc, gpp, nil
	}

	if _os == "darwin" && arch == "amd64" {
		osx := cleaveFolder().Concat("osxcross")
		if !osx.IsExtant() {
			yo.Warn("Darwin cross-compiler not installed. Will now attempt to install.")
			yo.Ok("Installing Mac OS toolchain...")
			script := findGopath().Concat("src", "github.com", "superp00t", "cleave", "bootstrap-osx-toolchain.sh")
			cmd := exec.Command("bash", script.Render())
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			err := cmd.Run()
			if err != nil {
				yo.Fatal(err)
			}
		}

		bin := osx.Concat("target", "bin")

		gcc := bin.Concat("o64-clang")
		if !gcc.IsExtant() {
			return "", "", fmt.Errorf("o64-clang missing")
		}

		yo.Warn(gcc)

		gpp := bin.Concat("o64-clang++")
		if !gpp.IsExtant() {
			return "", "", fmt.Errorf("o64-clang++ missing")
		}

		yo.Warn(gcc)
		yo.Warn(gpp)

		return gcc.Render(), gpp.Render(), nil
	}

	if _os == "windows" && arch == "amd64" {
		p, err := exec.LookPath("x86_64-w64-mingw32-gcc")
		if err != nil {
			return "", "", err
		}

		cp, err := exec.LookPath("x86_64-w64-mingw32-g++")
		if err != nil {
			return "", "", err
		}

		return p, cp, nil
	}

	return "", "", fmt.Errorf("no compilers found for %s-%s", _os, arch)
}

func CmakeFlags(platform, cc, cxx string) []string {
	str := strings.Split(platform, "-")
	if len(str) != 2 {
		return []string{}
	}

	args := []string{
		"-D",
		"CMAKE_C_COMPILER=" + cc,
		"-D",
		"CMAKE_CXX_COMPILER=" + cxx,
		"-D",
		"CMAKE_SYSTEM_NAME=" + cmakeOS(str[0]),
		"-D",
		"CMAKE_SYSTEM_PROCESSOR=" + cmakeArch(str[1]),
	}

	if str[0] == "windows" {
		args = append(args, "-D", "CMAKE_RC_COMPILER=x86_64-w64-mingw32-windres")
	}

	if str[0] == "darwin" {
		args = append(args, "-D", "CMAKE_OSX_SYSROOT="+cleaveFolder().Concat("osxcross", "target", "SDK", "MacOSX10.11.sdk").Render())
		args = append(args, "-D", "CMAKE_OSX_DEPLOYMENT_TARGET=10.11")
		args = append(args, "-D", "CMAKE_FIND_ROOT_PATH="+cleaveFolder().Concat("osxcross", "target", "macports", "pkgs", "opt", "local").Render())
		args = append(args, "-D", "CMAKE_AR="+cleaveFolder().Concat("osxcross", "target", "bin", "x86_64-apple-darwin15-ar").Render())
	}

	return args
}

func cmakeOS(str string) string {
	switch str {
	case "linux":
		return "Linux"
	case "darwin":
		return "Darwin"
	case "windows":
		return "Windows"
	}

	return str
}

func cmakeArch(str string) string {
	switch str {
	case "amd64":
		return "x86_64"
	case "i386":
		return "i386"
	case "i686":
		return "i386"
	}

	return str
}
