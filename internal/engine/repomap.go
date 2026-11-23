package engine

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
)

// Symbol 是仓库地图里的一个符号（函数/类型/方法）。
type Symbol struct {
	File      string
	Name      string
	Signature string
	Line      int
	Kind      string
	Refs      int
}

// RepoMap 是符号索引，按被引用次数排序后渲染成紧凑文本。
type RepoMap struct {
	Symbols []Symbol
	Tokens  int
}

var (
	pyDef      = regexp.MustCompile(`^\s*(?:async\s+)?def\s+([A-Za-z_]\w*)\s*\(`)
	pyClass    = regexp.MustCompile(`^\s*class\s+([A-Za-z_]\w*)`)
	jsFunc     = regexp.MustCompile(`^\s*(?:export\s+)?(?:async\s+)?function\s+([A-Za-z_$]\w*)\s*\(`)
	jsClass    = regexp.MustCompile(`^\s*(?:export\s+)?class\s+([A-Za-z_$]\w*)`)
	goFn       = regexp.MustCompile(`^\s*func\s+(?:\([^)]*\)\s*)?([A-Za-z_]\w*)\s*\(`)
	goType     = regexp.MustCompile(`^\s*type\s+([A-Za-z_]\w*)\s`)
	rustFn     = regexp.MustCompile(`^\s*(?:pub\s+)?(?:async\s+)?fn\s+([A-Za-z_]\w*)\s*\(`)
	rustStruct = regexp.MustCompile(`^\s*(?:pub\s+)?struct\s+([A-Za-z_]\w*)`)
)

var mapIgnoredDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "bin": true,
	"dist": true, ".idea": true, "__pycache__": true, ".hb-crash": true,
	"testdata": true, "assets": true,
}

// BuildRepoMap 扫描工作区，最多使用 maxTokens 的预算。
func BuildRepoMap(workdir string, maxTokens int) *RepoMap {
	if maxTokens <= 0 {
		maxTokens = 1200
	}
	m := &RepoMap{}
	var goFiles []string
	_ = filepath.WalkDir(workdir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, rerr := filepath.Rel(workdir, path)
		if rerr != nil || rel == "." {
			return nil
		}
		if d.IsDir() {
			if mapIgnoredDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(rel))
		if ext != ".go" && ext != ".py" && ext != ".js" && ext != ".ts" && ext != ".tsx" && ext != ".rs" && ext != ".java" {
			return nil
		}
		if ext == ".go" {
			goFiles = append(goFiles, path)
		}
		parseGenericFile(m, workdir, path, rel, ext)
		return nil
	})
	for _, path := range goFiles {
		rel, _ := filepath.Rel(workdir, path)
		parseGoFile(m, filepath.ToSlash(rel), path)
	}

	// 统计引用：所有符号名在所有源文件中的出现次数。
	countRefs(m, workdir)

	sort.SliceStable(m.Symbols, func(i, j int) bool {
		if m.Symbols[i].Refs != m.Symbols[j].Refs {
			return m.Symbols[i].Refs > m.Symbols[j].Refs
		}
		if m.Symbols[i].File != m.Symbols[j].File {
			return m.Symbols[i].File < m.Symbols[j].File
		}
		return m.Symbols[i].Line < m.Symbols[j].Line
	})

	// 控制 token 预算。
	var kept []Symbol
	for _, s := range m.Symbols {
		cost := core.TokenEstimate(s.Signature) + 3
		if m.Tokens+cost > maxTokens {
			break
		}
		m.Tokens += cost
		kept = append(kept, s)
	}
	m.Symbols = kept
	return m
}

func parseGoFile(m *RepoMap, rel, path string) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return
	}
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			var buf bytes.Buffer
			_ = printer.Fprint(&buf, fset, d.Type)
			sig := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(buf.String()), "func"))
			kind := "func"
			fullName := d.Name.Name
			if d.Recv != nil && len(d.Recv.List) > 0 {
				kind = "method"
				fullName = receiverName(d.Recv.List[0].Type) + "." + d.Name.Name
			}
			m.Symbols = append(m.Symbols, Symbol{
				File: rel, Name: d.Name.Name, Signature: fullName + sig,
				Line: fset.Position(d.Pos()).Line, Kind: kind,
			})
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				if ts, ok := spec.(*ast.TypeSpec); ok {
					m.Symbols = append(m.Symbols, Symbol{
						File: rel, Name: ts.Name.Name, Signature: "type " + ts.Name.Name,
						Line: fset.Position(ts.Pos()).Line, Kind: "type",
					})
				}
			}
		}
	}
}

// receiverName 提取方法接收者的类型名（去掉指针与泛型参数）。
func receiverName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.StarExpr:
		return receiverName(e.X)
	case *ast.IndexExpr:
		return receiverName(e.X)
	case *ast.IndexListExpr:
		return receiverName(e.X)
	case *ast.Ident:
		return e.Name
	default:
		return ""
	}
}

func parseGenericFile(m *RepoMap, workdir, path, rel, ext string) {
	if ext == ".go" {
		return // Go 用 AST 解析
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) > 2<<20 {
		return
	}
	var patterns []*regexp.Regexp
	var kind string
	switch ext {
	case ".py":
		patterns = []*regexp.Regexp{pyDef, pyClass}
		kind = "py"
	case ".js", ".ts", ".tsx":
		patterns = []*regexp.Regexp{jsFunc, jsClass}
		kind = "js"
	case ".rs":
		patterns = []*regexp.Regexp{rustFn, rustStruct}
		kind = "rs"
	default:
		return
	}
	for i, line := range strings.Split(string(data), "\n") {
		for _, re := range patterns {
			if mm := re.FindStringSubmatch(line); len(mm) > 1 {
				m.Symbols = append(m.Symbols, Symbol{
					File: filepath.ToSlash(rel), Name: mm[1],
					Signature: strings.TrimSpace(line), Line: i + 1, Kind: kind,
				})
			}
		}
	}
}

func countRefs(m *RepoMap, workdir string) {
	names := make(map[string]int)
	for _, s := range m.Symbols {
		names[s.Name]++
	}
	if len(names) == 0 {
		return
	}
	_ = filepath.WalkDir(workdir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".go" && ext != ".py" && ext != ".js" && ext != ".ts" && ext != ".tsx" && ext != ".rs" && ext != ".java" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil || len(data) > 2<<20 {
			return nil
		}
		text := string(data)
		for name := range names {
			m.Symbols = countOne(m.Symbols, name, strings.Count(text, name))
		}
		return nil
	})
}

func countOne(symbols []Symbol, name string, delta int) []Symbol {
	for i := range symbols {
		if symbols[i].Name == name {
			symbols[i].Refs += delta
		}
	}
	return symbols
}

// Render 输出紧凑的仓库地图文本，按文件分组。
func (m *RepoMap) Render() string {
	if m == nil || len(m.Symbols) == 0 {
		return ""
	}
	byFile := make(map[string][]Symbol)
	for _, s := range m.Symbols {
		byFile[s.File] = append(byFile[s.File], s)
	}
	files := make([]string, 0, len(byFile))
	for f := range byFile {
		files = append(files, f)
	}
	sort.Strings(files)
	var b strings.Builder
	for _, f := range files {
		fmt.Fprintf(&b, "%s:\n", f)
		for _, s := range byFile[f] {
			sig := s.Signature
			r := []rune(sig)
			if len(r) > 110 {
				sig = string(r[:110]) + "…"
			}
			fmt.Fprintf(&b, "  %s  [refs=%d]\n", sig, s.Refs)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}
