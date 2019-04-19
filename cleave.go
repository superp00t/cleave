package cleave

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/superp00t/etc"
	"github.com/superp00t/etc/yo"
)

const folderName = "CleaveCache"
const configName = "CleaveFile"

type srcImportReference struct {
	SourceFile string
	ImportID   string
}

type Target struct {
	Name       string                 `json:"name"`
	Type       string                 `json:"type"`
	Platform   string                 `json:"platform"`
	Production bool                   `json:"production"` // Strip debugging symbols and invoke UPX compression
	Flags      map[string]interface{} `json:"define"`
	Link       []string               `json:"link"`
	Main       string                 `json:"main"`
}

type PkgConfig struct {
	Path     etc.Path `json:"-"`
	Out      etc.Path `json:"-"`
	Name     string   `json:"name"`
	Author   string   `json:"author"`
	Desc     string   `json:"description"`
	Version  string   `json:"version"`
	Type     string   `json:"type"`
	Index    string   `json:"index"`
	Language string   `json:"language"`

	Src     []string `json:"src"`
	Include []string `json:"include"`

	Targets []Target          `json:"targets"`
	Deps    map[string]string `json:"deps"`

	Flags map[string][]string `json:"flags"`

	Icon   string   `json:"icon"`
	Dylibs []string `json:"dylibs"`

	currentFile string                         `json:"-"`
	luaImports  map[string]*srcImportReference `json:"-"`

	Test bool `json:"-"`
}

func cleaveFolder() etc.Path {
	cp := etc.LocalDirectory().Concat("cleave")
	if !cp.IsExtant() {
		os.MkdirAll(cp.Render(), 0700)
	}

	if !cp.Exists("Deps") {
		cp.Mkdir("Deps")
	}

	if !cp.Exists("Usr") {
		cp.Mkdir("Usr")
	}

	return cp
}

func LoadPackage(path string) (*PkgConfig, error) {
	pth := etc.ParseSystemPath(path)

	if !pth.Exists(configName) {
		return nil, fmt.Errorf("You do not have a cleave file.")
	}

	f, err := pth.Get(configName)
	if err != nil {
		return nil, err
	}

	pc := new(PkgConfig)
	pc.Path = pth
	err = json.NewDecoder(f).Decode(pc)
	if err != nil {
		return nil, err
	}

	out := []Target{}

	for _, v := range pc.Targets {
		if v.Name == "" {
			v.Name = pc.Name
		}

		platforms := strings.Split(v.Platform, ",")
		for _, platform := range platforms {
			t := v
			t.Platform = platform
			out = append(out, t)
		}
	}
	pc.Targets = out

	if len(pc.Deps) > 0 {
		if !pth.Exists(folderName) {
			pth.Mkdir(folderName)
		}

		for k, v := range pc.Deps {
			name := k
			url := v

			if strings.HasPrefix(url, "https://") {
				gp := name
				if !validKey(gp) {
					yo.Fatal("Invalid package name", gp)
				}

				if strings.HasSuffix(gp, ".git") {
					gp = strings.TrimRight(gp, ".git")
				}

				clf := cleaveFolder().Concat("Deps", gp)

				if !clf.IsExtant() {
					yo.Println("Retrieving git repository", gp)
					cmd := exec.Command("git", "clone", "--recursive", url, clf.Render())
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					err := cmd.Run()
					if err != nil {
						yo.Fatal(err)
					}
				} else {
					yo.Println("Updating git repository", gp)
					exec.Command("git", "pull", "-C", clf.Render()).Run()
				}
			}
		}
	}

	return pc, nil
}
