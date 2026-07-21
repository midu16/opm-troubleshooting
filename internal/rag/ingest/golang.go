package ingest

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/midu16/opm-troubleshooting/internal/rag"
)

// sourceDirs lists the top-level directories we scan inside each repo.
var sourceDirs = []string{
	"internal", "api", "pkg", "cmd", "controllers",
}

// ChunkGoSource walks the relevant subdirectories of repoDir, parses each
// .go file with go/parser, and emits one Document per top-level declaration
// (function, type, const, var) together with its doc comments.
func ChunkGoSource(repoDir string, repoName string) ([]rag.Document, error) {
	var docs []rag.Document

	for _, sub := range sourceDirs {
		subDir := filepath.Join(repoDir, sub)
		if _, err := os.Stat(subDir); err != nil {
			continue // directory does not exist in this repo
		}

		err := filepath.Walk(subDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				name := info.Name()
				if name == "vendor" || name == ".git" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			// Skip test files and generated files.
			base := filepath.Base(path)
			if strings.HasSuffix(base, "_test.go") {
				return nil
			}
			if strings.HasPrefix(base, "zz_generated") {
				return nil
			}

			fileDocs, parseErr := parseGoFile(path, repoDir, repoName)
			if parseErr != nil {
				// Log warning but continue.
				fmt.Fprintf(os.Stderr, "  warning: parse %s: %v\n", path, parseErr)
				return nil
			}
			docs = append(docs, fileDocs...)
			return nil
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "  warning: walking %s in %s: %v\n", sub, repoName, err)
		}
	}
	return docs, nil
}

// parseGoFile parses a single Go source file and extracts top-level
// declarations as individual documents.
func parseGoFile(path, repoDir, repoName string) ([]rag.Document, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	relPath, _ := filepath.Rel(repoDir, path)
	pkgName := f.Name.Name
	repoURL := "https://github.com/openshift/" + repoName

	var docs []rag.Document

	for _, decl := range f.Decls {
		var buf bytes.Buffer
		var declSummary string

		switch d := decl.(type) {
		case *ast.FuncDecl:
			// Include doc comment.
			if d.Doc != nil {
				for _, c := range d.Doc.List {
					buf.WriteString(c.Text)
					buf.WriteString("\n")
				}
			}
			if err := printer.Fprint(&buf, fset, d); err != nil {
				continue
			}
			declSummary = funcDeclSummary(d)

		case *ast.GenDecl:
			// Only process type, const, var (not import).
			if d.Tok == token.IMPORT {
				continue
			}
			if d.Doc != nil {
				for _, c := range d.Doc.List {
					buf.WriteString(c.Text)
					buf.WriteString("\n")
				}
			}
			if err := printer.Fprint(&buf, fset, d); err != nil {
				continue
			}
			declSummary = genDeclSummary(d)

		default:
			continue
		}

		content := buf.String()
		if strings.TrimSpace(content) == "" {
			continue
		}

		hash := sha256.Sum256([]byte(repoName + ":" + relPath + ":" + content))
		id := fmt.Sprintf("go-%s-%x", repoName, hash[:12])

		// Truncate declaration summary to 200 chars.
		if len(declSummary) > 200 {
			declSummary = declSummary[:200]
		}

		docs = append(docs, rag.Document{
			ID:      id,
			Content: content,
			Metadata: map[string]string{
				"source":      relPath,
				"type":        "go",
				"package":     pkgName,
				"declaration": declSummary,
				"repo":        repoName,
				"repo_url":    repoURL,
			},
		})
	}
	return docs, nil
}

// funcDeclSummary returns the first line of a function declaration.
func funcDeclSummary(d *ast.FuncDecl) string {
	var buf bytes.Buffer
	buf.WriteString("func ")
	if d.Recv != nil && len(d.Recv.List) > 0 {
		buf.WriteString("(")
		printer.Fprint(&buf, token.NewFileSet(), d.Recv.List[0].Type)
		buf.WriteString(") ")
	}
	buf.WriteString(d.Name.Name)
	buf.WriteString("(")
	if d.Type.Params != nil {
		for i, p := range d.Type.Params.List {
			if i > 0 {
				buf.WriteString(", ")
			}
			for j, n := range p.Names {
				if j > 0 {
					buf.WriteString(", ")
				}
				buf.WriteString(n.Name)
			}
			buf.WriteString(" ")
			printer.Fprint(&buf, token.NewFileSet(), p.Type)
		}
	}
	buf.WriteString(")")
	return buf.String()
}

// genDeclSummary returns a brief summary of a GenDecl (type/const/var).
func genDeclSummary(d *ast.GenDecl) string {
	s := d.Tok.String()
	if len(d.Specs) > 0 {
		switch spec := d.Specs[0].(type) {
		case *ast.TypeSpec:
			s += " " + spec.Name.Name
		case *ast.ValueSpec:
			if len(spec.Names) > 0 {
				s += " " + spec.Names[0].Name
			}
		}
	}
	return s
}
