package main

import (
	"os"
	"os/exec"

	"fmt"
	"runtime"
	"strings"

	"github.com/superp00t/cleave"
	"github.com/superp00t/etc"
	"github.com/superp00t/etc/yo"
)

func getPath() string {
	if sc := yo.StringG("s"); sc != "" {
		return sc
	}

	wd, err := os.Getwd()
	if err != nil {
		yo.Fatal(err)
	}

	return wd
}

func getOut() string {
	if sc := yo.StringG("o"); sc != "" {
		return sc
	}

	wd, err := os.Getwd()
	if err != nil {
		yo.Fatal(err)
	}

	return wd
}

func cpus() string {
	c := runtime.NumCPU()

	using := 0

	if c > 4 {
		using = 4
	} else if c < 4 && c >= 2 {
		using = 2
	} else {
		using = 1
	}

	str := fmt.Sprintf("-d%d", using)
	yo.Ok("Compiling with ", str, "CPU cores")
	return str
}

func cfu(s []string, test bool) {
	wd := getPath()
	wdir := etc.ParseSystemPath(wd)
	if !wdir.Exists("CleaveFile") && wdir.Exists("CMakeLists.txt") {
		cc, cppc, err := cleave.GetPlatformCompilers(yo.StringG("t"))
		if err != nil {
			yo.Fatal(err)
		}

		args := cleave.CmakeFlags(yo.StringG("t"), cc, cppc)
		if prefix := yo.StringG("p"); prefix != "" {
			args = append(args, "-D", "CMAKE_INSTALL_PREFIX="+prefix)
		}

		if strings.HasPrefix(yo.StringG("t"), "darwin-") {
			args = append(args, "-D", "CMAKE_OSX_DEPLOYMENT_TARGET=10.11")
		}

		c := exec.Command("mkdir", wdir.Concat("build").Render())
		c.Run()
		c = exec.Command("cmake", append(args, "..")...)
		c.Dir = wdir.Concat("build").Render()

		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		c.Run()

		c = exec.Command("make", cpus())
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		c.Dir = wdir.Concat("build").Render()

		c = exec.Command("make", "install")
		c.Dir = wdir.Concat("build").Render()
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		c.Run()

		return
	}

	pk, err := cleave.LoadPackage(wd)
	if err != nil {
		yo.Fatal(err)
	}
	pk.Test = test
	yo.Ok("test?", pk.Test)
	pk.Out = etc.ParseSystemPath(getOut())
	if err := pk.BuildC(); err != nil {
		yo.Fatal(err)
	}
}

func luafu(s []string) {
	wd, _ := os.Getwd()
	cfg, err := cleave.LoadPackage(wd)
	if err != nil {
		yo.Fatal(err)
	}

	if err := cfg.BuildCC(); err != nil {
		yo.Fatal(err)
	}
	os.Exit(0)
}

func main() {
	yo.Stringf("t", "target-platform", "the platform you are building for.", "")
	yo.Stringf("s", "source", "Source directory", "")
	yo.Stringf("o", "out", "Target compilation output directory", ".")
	yo.Stringf("p", "prefix", "Tells where to --install to", "")

	yo.Boolf("v", "verbose", "Shows additional logging details")
	yo.Boolf("i", "install", "Install compilation targets to a specified --prefix")

	yo.Main("A build process automation robot and dependency exorcist", func(args []string) {
		cfu(args, false)
	})

	yo.AddSubroutine("test", nil, "", func(args []string) {
		cfu(args, true)
	})

	yo.AddSubroutine("c-fu", nil, "Build a C project", func(args []string) {
		cfu(args, false)
	})

	yo.AddSubroutine("lua-fu", nil, "Build a ComputerCraft Lua project", luafu)

	yo.Init()
}
