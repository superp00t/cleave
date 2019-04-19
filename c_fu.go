package cleave

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image/png"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/superp00t/etc"
	"github.com/superp00t/etc/yo"

	ico "github.com/biessek/golang-ico"
)

func (p *PkgConfig) BuildC() error {
	for _, v := range p.Targets {
		if p.Test {
			if v.Type == "test" {
				if err := p.compileTarget(v); err != nil {
					yo.Warn(err)
				}
			}
		} else {
			if v.Type != "test" {
				if err := p.compileTarget(v); err != nil {
					yo.Warn(err)
				}
			}
		}
	}

	return nil
}

func (t Target) defineFlags() []string {
	args := []string{}
	for k, v := range t.Flags {
		switch v.(type) {
		case string:
			args = append(args, "-D"+k+"="+v.(string))
		case bool:
			if v.(bool) == true {
				args = append(args, "-D"+k)
			} else {
				args = append(args, "-D"+k+"=0")
			}
		}
	}
	sort.Strings(args)
	return args
}

func hashFile(i string) string {
	e, err := ioutil.ReadFile(i)
	if err != nil {
		yo.Fatal(err)
	}

	sh := sha256.New()
	sh.Write(e)

	return strings.ToUpper(hex.EncodeToString(sh.Sum(nil)))
}

func (t Target) ReleaseType() string {
	if t.Production == true {
		return "production"
	}

	return "testing"
}

// func acquirePlatformPrefix(platform string) string {
// 	switch platform {
// 	case "darwin-amd64":
// 		return cleaveFolder().Concat("o")
// 	}
// }

func (p *PkgConfig) compileTarget(t Target) error {
	if t.Type == "test" {
		t.Platform = runtime.GOOS + "-" + runtime.GOARCH
	}

	for k := range p.Deps {
		usr := cleaveFolder().Concat(t.Platform, "Usr")
		if !usr.IsExtant() {
			err := os.MkdirAll(usr.Render(), 0700)
			if err != nil {
				return err
			}
		}
		yo.Println("compiling", k, "for", t.Platform, "to", usr.Render())
		cmd := exec.Command(os.Args[0], "c-fu", "--target-platform", t.Platform, "--prefix", usr.Render())
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Dir = cleaveFolder().Concat("Deps", k).Render()
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	mn := t.Main
	language := p.Language
	pth := etc.ParseUnixPath(mn)
	mn = pth.RenderWin()

	if language == "" {
		language = "c"
	}

	// Build Cgo program
	if language == "go" {
		pkg := strings.Split(t.Main, ":")[1]

		cc, cxx, err := GetPlatformCompilers(t.Platform)
		if err != nil {
			return err
		}

		strs := strings.Split(t.Platform, "-")

		exeName := t.Name + "-" + p.Version + "-" + t.Platform + "-" + t.ReleaseType()

		yo.Println("Building", pkg, exeName)
		buildCmd := exec.Command("go", "build", "-o", exeName, pkg)
		buildCmd.Env = append(os.Environ(), []string{
			"MACOSX_DEPLOYMENT_TARGET=10.11",
			"CGO_ENABLED=1",
		}...)

		// if runtime.GOOS != strs[0] && runtime.GOARCH != strs[1] {
		buildCmd.Env = append([]string{
			"GOOS=" + strs[0],
			"GOARCH=" + strs[1],
			"CC=" + cc,
			"CXX=" + cxx,
			"CGO_CFLAGS=-I" + cleaveFolder().Concat(t.Platform, "Usr", "include").Render(),
			"CGO_LDFLAGS=-L" + cleaveFolder().Concat(t.Platform, "Usr", "lib").Render(),
			"PKG_CONFIG_PATH=" + cleaveFolder().Concat(t.Platform, "Usr", "lib", "pkgconfig").Render(),
		}, buildCmd.Env...)

		if strs[0] == "darwin" {
			buildCmd.Env = append(buildCmd.Env, "PATH="+os.Getenv("PATH")+":"+cleaveFolder().Concat("osxcross", "target", "bin").Render())
		}

		yo.Ok("Platform: ", t.Platform)
		yo.Ok("C Compiler: ", cc)
		// }

		buildCmd.Stderr = os.Stderr
		buildCmd.Stdout = os.Stdout

		if err := buildCmd.Run(); err != nil {
			yo.Fatal(err)
		}
		return nil
	}

	if language == "c" || language == "c++" {
		oa := strings.Split(t.Platform, "-")
		if len(oa) != 2 {
			return fmt.Errorf("Invalid platform formatting")
		}

		platformFlags := p.Flags[t.Platform]
		if len(platformFlags) == 0 {
			if els := strings.Split(t.Platform, "-"); len(els) == 2 {
				platformFlags = p.Flags[els[0]]
			}
		}

		for _, v := range p.Flags["*"] {
			platformFlags = append(platformFlags, v)
		}

		cmp, ccCmp, err := acquireCompiler(oa[0], oa[1])
		if err != nil {
			return err
		}

		if language == "c++" {
			cmp = ccCmp
		}

		var srcs []string
		var incl []string

		for _, v := range p.Src {
			purportedFolders := p.Path.Concat(etc.ParseUnixPath(v)...).Render()
			yo.Spew(purportedFolders)
			psrcs, err := filepath.Glob(purportedFolders)
			if err != nil {
				return err
			}
			srcs = append(srcs, psrcs...)
		}

		for _, v := range p.Include {
			incl = append(incl, v)
		}

		// if len(srcs) == 0 {
		// 	return fmt.Errorf("No C source files to compile")
		// }

		tgt := etc.Path{folderName, "Target", "Objects", t.Platform}
		if !p.Out.ExistsPath(tgt) {
			p.Out.MakeDirPath(tgt)
		}

		tgto := p.Out.GetSub(tgt)

		stderr := etc.NewBuffer()

		rType := "testing"
		if t.Production {
			rType = "production"
		}

		objectFiles := []string{}

		lenFiles := float64(len(srcs))
		fileRatio := 1 / lenFiles

		pct := 0.0

		successfulOutputs := []string{}

		for _, v := range srcs {
			tf := etc.ParseSystemPath(v)
			topFile, _ := tf.Pop()
			topFileEl := strings.Split(topFile, ".")
			if len(topFileEl) != 2 {
				yo.Fatal("invalid source name", v)
			}

			topFile = topFileEl[0]

			obj := topFile + "-" + t.Platform + "-" + rType + ".o"

			alreadyCompiled := tgto.Concat(obj).Render() + ".*"
			ex, err := filepath.Glob(alreadyCompiled)
			if err != nil {
				yo.Fatal(err)
			}

			var srcHash = hashFile(v)

			if len(ex) != 0 {
				sfx := strings.Split(ex[0], ".")
				hash := sfx[len(sfx)-1]

				if srcHash == hash {
					pct += (fileRatio * 100)
					objectFiles = append(objectFiles, tgto.Concat(obj).Render())
					continue
				} else {
					os.Remove(ex[0])
				}
			}

			objectFiles = append(objectFiles, tgto.Concat(obj).Render())

			args := []string{
				"-c",
				v,
			}

			if rType == "testing" {
				args = append(args, "-ggdb")
			}
			for _, i := range incl {
				args = append(args, "-I"+p.Path.Concat(i).Render())
			}

			for _, v := range platformFlags {
				args = append(args, v)
			}

			args = append(args, "-o")
			args = append(args, tgto.Concat(obj).Render())

			args = append(args, t.defineFlags()...)

			for _, v := range t.Link {
				args = append(args, v)
			}

			yo.Spew(args)

			// yo.Println(cmp, args)

			cmd := exec.Command(cmp, args...)
			cmd.Stderr = stderr
			cmd.Stdout = stderr

			err = cmd.Run()
			if err != nil {
				yo.Println(stderr.ToString())
				return err
			}

			yo.Println(stderr.ToString())

			yo.Printf("(%.2f%% done) Compiled %s\n", pct, obj)
			pct += (fileRatio * 100)

			h := tgto.Concat(obj).Render() + "." + srcHash
			err = ioutil.WriteFile(h, []byte("1"), 0700)
			if err != nil {
				yo.Fatal(err)
			}

			yo.Println("Wrote hash", h)
		}

		yo.Println("Object compilation successful! Linking", t.Platform, t.Name)

		if t.Type == "static-lib" {
			c := "ar"
			arArgs := []string{"rcs", p.Out.Concat("lib" + p.Name + "-" + p.Version + "-" + t.Platform + "-" + rType + ".a").Render()}

			for _, v := range objectFiles {
				arArgs = append(arArgs, v)
			}

			ex := exec.Command(c, arArgs...)
			if err := ex.Run(); err != nil {
				return err
			}

			yo.Println("Compiled static library @" + arArgs[1])
			if t.Platform == runtime.GOOS+"-"+runtime.GOARCH {
				successfulOutputs = append(successfulOutputs, arArgs[1])
			}
		}

		if t.Type == "exe" || t.Type == "test" {

			var baseFolder etc.Path
			var srcFolder etc.Path
			if srcFolder = etc.ParseSystemPath(mn); len(srcFolder) > 1 {
				baseFolder = srcFolder[:len(srcFolder)-1]
			}

			fname, _ := srcFolder.Pop()

			objectsPath := p.Out.Concat("CleaveCache", "Target", "Objects", t.Platform)
			if len(baseFolder) > 1 {
				objectsPath.Concat(baseFolder...).MakeDir()
			}

			mainObjectPath := objectsPath.Concat(fname + ".o").Render()
			mainArgs := []string{
				"-c",
				p.Path.GetSub(etc.ParseUnixPath(mn)).Render(),
				"-o",
				mainObjectPath}

			if rType == "testing" {
				mainArgs = append([]string{"-ggdb"}, mainArgs...)
			}

			mainArgs = append(mainArgs, t.defineFlags()...)

			for _, v := range incl {
				mainArgs = append([]string{
					"-I",
					p.Path.Concat(v).Render(),
				},
					mainArgs...)
			}

			mainCmd := exec.Command(cmp, mainArgs...)
			mainCmd.Stdout = stderr
			mainCmd.Stderr = stderr

			yo.Spew(mainArgs)

			err = mainCmd.Run()
			if err != nil {
				return errors.New(stderr.ToString())
			}
			gcArgs := []string{mainObjectPath}
			if t.Production == false {
				gcArgs = append([]string{"-ggdb"}, gcArgs...)
			}

			for _, v := range objectFiles {
				gcArgs = append(gcArgs, v)
			}

			exeFile := ""
			if p.Test {
				exeFile = "unittest-" + t.Name + rType
			} else {
				exeFile = t.Name + "-" + p.Version + "-" + t.Platform + "-" + rType
			}
			if oa[0] == "windows" {
				exeFile += ".exe"

				if p.Out.Concat("CleaveIcon.png").IsExtant() {
					yo.Ok("generating icon")

					rd, err := p.Out.Concat("CleaveIcon.png").ReadAll()
					if err != nil {
						yo.Fatal(err)
					}

					buf := etc.FromBytes(rd)

					mg, err := png.Decode(buf)
					if err != nil {
						yo.Fatal(err)
					}

					out := etc.NewBuffer()
					ico.Encode(out, mg)

					icoFile := p.Out.Concat("CleaveCache", "Target", "icon.ico")
					icoFile.WriteAll(out.Bytes())

					relPath := p.Out.Concat("CleaveCache", "Target", "icon.rel").Render()

					c := exec.Command("rsrc", "-arch="+t.Arch(), "-ico="+icoFile.Render(), "-o="+relPath)
					if err := c.Run(); err != nil {
						yo.Warn(err)
					} else {
						gcArgs = append(gcArgs, relPath)
					}
				}

			}

			gcArgs = append(gcArgs, "-o")
			gcArgs = append(gcArgs, p.Out.Concat(exeFile).Render())
			for _, i := range incl {
				gcArgs = append(gcArgs, "-I"+p.Path.Concat(i).Render())
			}
			for _, v := range t.Link {
				gcArgs = append(gcArgs, v)
			}
			for _, v := range platformFlags {
				gcArgs = append(gcArgs, v)
			}
			yo.Spew(gcArgs)
			gcCmd := exec.Command(cmp, gcArgs...)
			gcCmd.Stderr = stderr
			gcCmd.Stdout = stderr
			err = gcCmd.Run()
			if err != nil {
				return errors.New(stderr.ToString())
			}

			if p.Test == false {
				yo.Ok("Compiled executable @" + exeFile)
			} else {
				yo.Ok("Running test...")
			}

			if t.Platform == runtime.GOOS+"-"+runtime.GOARCH {
				successfulOutputs = append(successfulOutputs, exeFile)
			}

			if t.Type == "test" {
				cmd := exec.Command(p.Out.Concat(exeFile).Render())
				cmd.Stderr = os.Stdout
				cmd.Stdout = os.Stdout
				cmd.Run()
			}

			return nil
		}

		if yo.BoolG("i") && t.Type != "test" {
			prefix := yo.StringG("p")

			if runtime.GOOS == "windows" {
				if prefix == "" {
					yo.Fatal("You must supply an installation prefix with --prefix")
				}
			} else {
				prefix = "/usr/local/"
			}

			pfx := etc.ParseSystemPath(prefix)

			for _, v := range successfulOutputs {
				t := "exe"
				if strings.HasSuffix(v, ".a") {
					t = "static-lib"
				}

				if t == "exe" {
					cp(v, pfx.Concat("bin", v).Render())
				}

				if t == "static-lib" {
					cp(v, pfx.Concat("lib", "lib"+p.Name+".a").Render())

					for _, v := range p.Include {
						mtch, err := filepath.Glob(p.Path.Concat(v, "*.h").Render())
						if err != nil {
							yo.Fatal(err)
						}

						for _, hfile := range mtch {
							cp(hfile, pfx.Concat("include").Render())
						}
					}
				}
			}

			return nil
		}
	}

	return fmt.Errorf("Unknown target type: %s", t.Type)
}

func cp(src, dest string) {
	if yo.BoolG("v") {
		yo.Warn("Copying", src, "to", dest)
	}

	et := etc.NewBuffer()
	e := exec.Command("cp", "-rf", src, dest)
	e.Stdout = et
	if err := e.Run(); err != nil {
		yo.Fatal(err)
	}

	if et.Len() > 0 {
		yo.Fatal("failed copy")
	}

	exec.Command("chmod", "-R", "455", dest).Run()
}

func (t Target) Arch() string {
	pl := strings.Split(t.Platform, "-")
	if len(pl) == 2 {
		return pl[1]
	}

	return "amd64"
}
