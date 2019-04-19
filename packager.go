package cleave

import (
	"html/template"
	"image"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/jackmordaunt/icns"
	"github.com/superp00t/etc"
	"github.com/superp00t/etc/yo"
)

func (pc *PkgConfig) packageExe(platform, executable, output string) {
	switch platform {
	case "darwin":
		pc.packageDarwin(executable, output)
	}
}

func (pc *PkgConfig) packageDarwin(executable, output string) {
	output += ".app"

	app := etc.ParseSystemPath(output)

	if app.IsExtant() {
		os.RemoveAll(app.Render())
	}

	os.MkdirAll(app.Render(), 0700)

	os.MkdirAll(app.Concat("Contents", "MacOS").Render(), 0700)
	os.MkdirAll(app.Concat("Contents", "Resources").Render(), 0700)
	os.MkdirAll(app.Concat("Contents", "Frameworks").Render(), 0700)

	if pc.Icon != "" {
		f, err := os.Open(pc.Icon)
		if err != nil {
			yo.Fatal(err)
		}

		img, _, err := image.Decode(f)
		if err != nil {
			yo.Fatal(err)
		}

		out := etc.NewBuffer()
		icns.Encode(out, img)

		ioutil.WriteFile(app.Concat("Contents", "Resources", "icon.icns").Render(), out.Bytes(), 0700)
	}

	move(executable, app.Concat("Contents", "MacOS", executable).Render())

	ioutil.WriteFile(app.Concat("Contents", "Info.plist").Render(), generatePlist(executable, "club.ikrypto.pg.cleave"), 0700)

	for _, v := range pc.Dylibs {
		cpy(cleaveFolder().Concat("darwin-amd64", "Usr", "lib", v).Render(), v)
	}
}

func move(src, dest string) {
	c := exec.Command("mv", src, dest)
	err := c.Run()
	if err != nil {
		yo.Fatal(err)
	}
}

func cpy(src, dest string) {
	c := exec.Command("cp", src, dest)
	err := c.Run()
	if err != nil {
		yo.Fatal(err)
	}
}

type plist struct {
	Exe string
	Id  string
}

const plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleExecutable</key>
	<string>{{.Exe}}</string>
	<key>CFBundleIconFile</key>
	<string>icon.icns</string>
	<key>CFBundleIdentifier</key>
	<string>{{.Id}}</string>
	<key>NSHighResolutionCapable</key>
	<true/>
	<key>LSUIElement</key>
	<true/>
</dict>
</plist>`

func generatePlist(exe, id string) []byte {
	pl := plist{
		Exe: exe, Id: id,
	}

	out := etc.NewBuffer()

	t, _ := template.New("").Parse(plistTemplate)
	t.Execute(out, pl)

	return out.Bytes()
}
