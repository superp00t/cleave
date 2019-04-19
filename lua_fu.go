package cleave

import (
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/superp00t/etc"
	"github.com/superp00t/etc/yo"

	"github.com/yuin/gopher-lua/ast"
	"github.com/yuin/gopher-lua/parse"
)

func validKey(s string) bool {
	r, err := regexp.Compile("^[_A-Za-z]{1}")
	if err != nil {
		panic(err)
	}

	return r.MatchString(s)
}

func isNumber(s string) bool {
	_, err := strconv.ParseFloat(s, 64)
	if err == nil {
		return true
	}

	return false
}

func (c *PkgConfig) encodeExpr(w io.Writer, exp []ast.Expr, name string) {
	for _, v := range exp {
		switch as := v.(type) {
		case *ast.StringExpr:
			if name == "NULL" && validKey(as.Value) {
				fmt.Fprint(w, as.Value)
			} else {
				fmt.Fprint(w, `"`)
				fmt.Fprint(w, as.Value)
				fmt.Fprint(w, `"`)
			}
		case *ast.UnaryLenOpExpr:
			fmt.Fprint(w, "#")
			c.encodeExpr(w, []ast.Expr{as.Expr}, "")
		case *ast.UnaryMinusOpExpr:
			fmt.Fprint(w, "-")
			c.encodeExpr(w, []ast.Expr{as.Expr}, "")
		case *ast.RelationalOpExpr:
			c.encodeExpr(w, []ast.Expr{as.Lhs}, "")
			fmt.Fprint(w, as.Operator)
			c.encodeExpr(w, []ast.Expr{as.Rhs}, "")

		case *ast.NilExpr:
			fmt.Fprintf(w, "nil")
		case *ast.TableExpr:
			fmt.Fprint(w, "{")
			for i, v := range as.Fields {
				if v.Key != nil {
					c.encodeExpr(w, []ast.Expr{v.Key}, "NULL")
					fmt.Fprint(w, "=")
				}
				c.encodeExpr(w, []ast.Expr{v.Value}, "")
				if i != len(as.Fields)-1 {
					fmt.Fprint(w, ",")
				}
			}
			fmt.Fprint(w, "}")
		case *ast.IdentExpr:
			fmt.Fprint(w, as.Value)
		case *ast.NumberExpr:
			// transform?
			fmt.Fprintf(w, "%s", as.Value)
		case *ast.StringConcatOpExpr:
			c.encodeExpr(w, []ast.Expr{as.Lhs}, "")
			fmt.Fprintf(w, "..")
			c.encodeExpr(w, []ast.Expr{as.Rhs}, "")
		case *ast.FunctionExpr:
			fmt.Fprintf(w, "function %s(", name)
			for i, v := range as.ParList.Names {
				fmt.Fprintf(w, "%s", v)
				if i != (len(as.ParList.Names) - 1) {
					fmt.Fprintf(w, ",")
				}
			}
			fmt.Fprintf(w, ")")
			c.encodeStmt(w, as.Stmts)
			fmt.Fprintf(w, "end")
		case *ast.FuncCallExpr:
			if as.Receiver != nil {
				fmt.Fprint(w, as.Receiver.(*ast.IdentExpr).Value)
				fmt.Fprint(w, ":")
			}

			fmt.Fprint(w, as.Method)
			if as.Method == "" {
				if aget, ok := as.Func.(*ast.AttrGetExpr); ok {
					if akey, ok2 := aget.Key.(*ast.StringExpr); ok2 {
						if akey.Value == "require" && aget.Object.(*ast.StringExpr).Value == "Cleave" {
							refGuid := etc.GenerateRandomUUID().String()
							if len(as.Args) > 0 {
								importString, ok := as.Args[0].(*ast.StringExpr)
								if ok {
									c.luaImports[importString.Value] = &srcImportReference{
										c.currentFile,
										refGuid,
									}
									fmt.Fprintf(w, "__i(%s)", refGuid)
									return
								}
							}
						}
					}
				}
				c.encodeExpr(w, []ast.Expr{as.Func}, "")
			}
			fmt.Fprint(w, "(")

			for i, v := range as.Args {
				c.encodeExpr(w, []ast.Expr{v}, "")
				if i != len(as.Args)-1 {
					fmt.Fprint(w, ",")
				}
			}
			fmt.Fprint(w, ")")
		case *ast.AttrGetExpr:
			c.encodeExpr(w, []ast.Expr{as.Object}, "")
			if nm, ok := as.Key.(*ast.NumberExpr); ok {
				fmt.Fprintf(w, "[%s]", nm.Value)
			} else {
				if str, ok := as.Key.(*ast.StringExpr); ok {
					if !validKey(str.Value) {
						fmt.Fprintf(w, "[%s]", str.Value)
					} else {
						fmt.Fprintf(w, ".%s", str.Value)
					}
				} else {
					fmt.Fprintf(w, "[")
					c.encodeExpr(w, []ast.Expr{as.Key}, "")
					fmt.Fprintf(w, "]")
				}
			}
		case *ast.ArithmeticOpExpr:
			fmt.Fprint(w, "(")
			c.encodeExpr(w, []ast.Expr{as.Lhs}, "")
			fmt.Fprint(w, as.Operator)
			c.encodeExpr(w, []ast.Expr{as.Rhs}, "")
			fmt.Fprint(w, ")")
		case *ast.LogicalOpExpr:
			c.encodeExpr(w, []ast.Expr{as.Lhs}, "")
			fmt.Fprintf(w, " %s ", as.Operator)
			c.encodeExpr(w, []ast.Expr{as.Rhs}, "")
		default:
			if as != nil {
				yo.Fatal(spew.Sdump(as))
			}
		}
	}
}

func (c *PkgConfig) encodeStmt(w io.Writer, st []ast.Stmt) {
	for _, v := range st {
		switch as := v.(type) {
		case *ast.AssignStmt:
			c.encodeExpr(w, as.Lhs, "")
			fmt.Fprint(w, "=", "")
			c.encodeExpr(w, as.Rhs, "")
		case *ast.ReturnStmt:
			fmt.Fprintf(w, "return")
			if len(as.Exprs) > 0 {
				fmt.Fprint(w, " ")
			}
			c.encodeExpr(w, as.Exprs, "")
		case *ast.GenericForStmt:
			fmt.Fprintf(w, "for %s in ", strings.Join(as.Names, ","))
			c.encodeExpr(w, as.Exprs, "")
			fmt.Fprintf(w, "do ")
			c.encodeStmt(w, as.Stmts)
			fmt.Fprint(w, "end")
		case *ast.RepeatStmt:
			fmt.Fprintf(w, "repeat ")
			c.encodeStmt(w, as.Stmts)
			fmt.Fprintf(w, "until ")
			c.encodeExpr(w, []ast.Expr{as.Condition}, "")
		case *ast.NumberForStmt:
			fmt.Fprintf(w, "for %s = ", as.Name)
			c.encodeExpr(w, []ast.Expr{as.Init}, "")
			fmt.Fprint(w, ",")
			c.encodeExpr(w, []ast.Expr{as.Limit}, "")
			if as.Step != nil {
				fmt.Fprint(w, ",")
				c.encodeExpr(w, []ast.Expr{as.Step}, "")
			}
			fmt.Fprintf(w, " do ")
			c.encodeStmt(w, as.Stmts)
			fmt.Fprint(w, "end")
		case *ast.WhileStmt:
			fmt.Fprintf(w, "while ")
			c.encodeExpr(w, []ast.Expr{as.Condition}, "")
			fmt.Fprintf(w, " do ")
			c.encodeStmt(w, as.Stmts)
			fmt.Fprintf(w, "end")
		case *ast.IfStmt:
			fmt.Fprintf(w, "if ")
			c.encodeExpr(w, []ast.Expr{as.Condition}, "")
			fmt.Fprintf(w, " then ")

			c.encodeStmt(w, as.Then)
			if len(as.Else) > 0 {
				fmt.Fprintf(w, "else ")
				c.encodeStmt(w, as.Else)
			}
			fmt.Fprintf(w, "end")
		case *ast.FuncCallStmt:
			c.encodeExpr(w, []ast.Expr{as.Expr}, "")
		case *ast.LocalAssignStmt:
			fmt.Fprintf(w, "local %s=", strings.Join(as.Names, ", "))
			c.encodeExpr(w, as.Exprs, "")
		case *ast.FuncDefStmt:
			nm := as.Name.Method
			ax, ok := as.Name.Receiver.(*ast.IdentExpr)
			if ok {
				nm = ax.Value + ":" + nm
			} else {
				nm = as.Name.Func.(*ast.IdentExpr).Value
			}

			c.encodeExpr(w, []ast.Expr{as.Func}, nm)
		}
		fmt.Fprintf(w, " ")
	}
}

func (c *PkgConfig) Minify(src string) string {
	lx, err := parse.Parse(strings.NewReader(src), "lxar")
	if err != nil {
		fmt.Println(err)
		return src
	}

	out := etc.NewBuffer()

	c.encodeStmt(out, lx)

	return out.ToString()
}

func (p *PkgConfig) extractCCreferences(path string) [][2]string {
	g := path + ".lua"
	if !p.Path.Exists(g) {
		return nil
	}

	f, err := p.Path.Get(g)
	if err != nil {
		return [][2]string{}
	}

	src := f.ToString()

	as, err := parse.Parse(strings.NewReader(src), "file.lua")
	if err != nil {
		yo.Fatal(err)
		return [][2]string{}
	}

	var refs [][2]string

	for _, v := range as {
		if e := luasearchStmt(v); e != "" {
			refs = append(refs, [2]string{
				p.Path.GetSub(etc.Path{path + ".lua"}).Render(),
				e})
		}
	}

	return refs
}

func luasearchExpr(a ast.Expr) string {
	switch as := a.(type) {

	case *ast.FuncCallExpr:
		if fnc, ok := as.Func.(*ast.AttrGetExpr); ok {
			if fnc.Object.(*ast.IdentExpr).Value == "Cleave" && fnc.Key.(*ast.StringExpr).Value == "require" {
				if len(as.Args) > 0 {
					ag := as.Args[0]
					se, ok := ag.(*ast.StringExpr)
					if !ok {
						return ""
					}

					return se.Value
				}
			}
		}
	case *ast.FunctionExpr:
		for _, v := range as.Stmts {
			if e, ok := v.(*ast.FuncCallStmt); ok {
				e := luasearchStmt(e)
				if e != "" {
					return e
				}
			}
		}

		return ""

	}

	return ""
}

func luasearchStmt(a ast.Stmt) string {
	switch as := a.(type) {
	case *ast.AssignStmt:
		for _, v := range as.Rhs {
			if e := luasearchExpr(v); e != "" {
				return e
			}
		}
	case *ast.LocalAssignStmt:
		for _, v := range as.Exprs {
			if e := luasearchExpr(v); e != "" {
				return e
			}
		}
	case *ast.FuncCallStmt:
		return luasearchExpr(as.Expr)
	case *ast.FuncDefStmt:
		return luasearchExpr(as.Func)
	}

	return ""
}

type ccDep struct {
	path string
	deps []*ccDep
}

func (p *PkgConfig) addDeps(m map[[2]string]int, path [2]string) {
	pths := p.extractCCreferences(path[0])
	for _, v := range pths {
		m[v] = m[v] + 1
		p.addDeps(m, v)
	}
}

func (pk *PkgConfig) TrueImportPath(src, p string) string {
	if p[0:2] == "./" {
		s := etc.ParseUnixPath(p[2:])
		//        some/path.lua      some  some/import.lua
		_, scc := etc.ParseUnixPath(src).Pop()
		sc := scc.Concat(s...).RenderUnix()
		return sc
	}
	expath := p
	m, err := regexp.MatchString("^module:(.*):(.*)", p)
	if err != nil {
		panic(err)
	}

	if m {
		ms := strings.Split(p, ":")
		expath = ms[2]
		return pk.Path.Concat(append([]string{"Cleave.modules", ms[1]}, etc.ParseUnixPath(expath)...)...).Render()
	} else {
		return p
	}
}

func (p *PkgConfig) GetAllImports() [][2]string {
	pth := make(map[[2]string]int)

	p.addDeps(pth, [2]string{p.Index, ""})

	var out [][2]string
	for k := range pth {
		out = append(out, k)
	}

	return out
}

func (p *PkgConfig) BuildCC() error {
	g := strings.Replace(etc.GenerateRandomUUID().String(), "-", "_", -1)
	dstLua := fmt.Sprintf(`-- Generated by Cleave
-- Package:     %s
-- Author:      %s
-- Version:     %s
-- Author:      %s
-- Compiled at: %s
-- UUID:        %s
`, p.Name, p.Author, p.Version, p.Desc, time.Now().Format(time.RFC3339), g)

	dstLua += fmt.Sprintf(`function p_%s()
__p={}
__i=function(p) return __p[p]() end
`, g)

	for _, v := range p.GetAllImports() {
		g := p.TrueImportPath(v[0], v[0]) + ".lua"
		if !etc.ParseSystemPath(g).ExistsPath(nil) {
			yo.Fatal("Could not load", v, "(", g, ")")
		}

		f, err := etc.ParseSystemPath(g).GetSubFile(nil)
		if err != nil {
			yo.Fatal("Could not load", g)
		}

		src := f.ToString()

		p.currentFile = g
		out := p.Minify(src)

		dstLua += fmt.Sprintf("cl.pkg[\"%s\"]=function()%send\n", v, out)
	}

	f, err := p.Path.Get(p.Index + ".lua")
	if err != nil {
		yo.Fatal("Could not load", p.Index+".lua")
	}

	src := f.ToString()

	out := p.Minify(src)

	dstLua += fmt.Sprintf("cl.pkg.main=function()%send\n", out)
	dstLua += fmt.Sprintf("cl.pkg.main()\n")
	dstLua += `end`
	dstLua += fmt.Sprintf("\np_%s()", g)

	fmt.Println(dstLua)
	return nil
}
