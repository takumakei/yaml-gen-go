// Package generator provides the function to convert YAML data into Go source code.
package generator

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/charmbracelet/glamour"
	"github.com/goaux/contextvalue"
	"github.com/goaux/iter/bufioscanner"
	"github.com/goaux/stacktrace/v2"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/takumakei/yaml-gen-go/execpipe"
	"gopkg.in/yaml.v3"
)

// Main is ...
func Main(ctx context.Context, config Config) {
	cmd := &cobra.Command{
		Use:     config.Use,
		Short:   config.Short,
		Long:    render(config.Long),
		Version: config.Version,
		RunE:    run,

		ValidArgsFunction: validArgs,

		SilenceErrors: true,
		SilenceUsage:  true,
	}

	fl := cmd.Flags()
	fl.SortFlags = false
	fl.StringVarP(&flags.Input, "in", "i", config.DefaultInput, "Input `filename.yaml`")
	fl.StringVarP(&flags.Output, "out", "o", config.DefaultOutput, "Output `filename.go`")
	fl.StringVarP(&flags.Package, "package", "p", config.DefaultPackage, "`package` name")
	fl.BoolVarP(&flags.Format, "format", "F", config.DefaultFormat, "Use goimports if true")

	cmd.CompletionOptions.HiddenDefaultCmd = true
	cmd.MarkFlagFilename("in", "yaml", "json")
	cmd.MarkFlagFilename("out", "go")
	cmd.RegisterFlagCompletionFunc("package", func(*cobra.Command, []string, string) ([]cobra.Completion, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	})

	ctx = contextvalue.With(ctx, &config)
	if err := cmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err.Error())
		os.Exit(1)
	}
}

func render(usage string) string {
	if isTTY(os.Stdout) {
		r, err := glamour.NewTermRenderer(
			glamour.WithEnvironmentConfig(),
			glamour.WithWordWrap(100),
		)
		if err == nil { // if NO error
			if s, err := r.Render(usage); err == nil { // if NO error
				return s
			}
		}
	}
	return usage
}

func validArgs(_ *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	return nil, cobra.ShellCompDirectiveNoFileComp
}

var flags flagsType

type flagsType struct {
	Input   string
	Output  string
	Package string
	Format  bool
}

func run(cmd *cobra.Command, args []string) error {
	config, ok := contextvalue.From[*Config](cmd.Context())
	if !ok {
		panic("never")
	}
	tmpl, err := template.New("").Funcs(funcs).Parse(config.Template)
	if err != nil {
		panic(err)
	}

	cin := cmd.InOrStdin()
	if flags.Input == "" && isTTY(cin) {
		return pflag.ErrHelp
	}

	if flags.Format {
		if err := execpipe.CheckPath("goimports"); err != nil {
			return errors.New("goimports was not found, consider using `--format=false`")
		}
	}

	input, data, err := readInputAuto(cin)
	if err != nil {
		return err
	}
	pkgname, err := readPackageAuto(input)
	if err != nil {
		return err
	}

	model := &Model{
		Gen: Gen{
			Name:    cmd.Name(),
			Version: cmd.Version,
		},
		Input: Input{
			Path: input,
			Data: data,
		},
		Output: Output{
			Package: pkgname,
		},
	}

	buf := new(bytes.Buffer)
	if err := tmpl.Execute(buf, model); err != nil {
		return err
	}

	if flags.Format {
		out := new(bytes.Buffer)
		if err := execpipe.Run(out, buf, "goimports"); err != nil {
			return err
		}
		buf = out
	}

	if input == "(stdin)" {
		_, err := cmd.OutOrStdout().Write(buf.Bytes())
		return err
	}
	ext := filepath.Ext(input)
	output := input[:len(input)-len(ext)] + ".go"
	return os.WriteFile(output, buf.Bytes(), 0644)
}

var funcs = map[string]any{
	"basename": filepath.Base,
	"dirname":  filepath.Dir,
	"abs":      filepath.Abs,

	"jsonify": jsonify,
}

func jsonify(v any) (string, error) {
	buf := new(bytes.Buffer)
	je := json.NewEncoder(buf)
	je.SetEscapeHTML(false)
	je.SetIndent("", "  ")
	err := je.Encode(v)
	return buf.String(), err
}

func isTTY(io any) bool {
	if f, ok := io.(*os.File); ok {
		return isatty.IsTerminal(f.Fd())
	}
	return false
}

func readInputAuto(cin io.Reader) (path string, data any, err error) {
	if flags.Input == "" {
		data, err := readInput(cin)
		return "(stdin)", data, err
	}
	path, err = filepath.Abs(flags.Input)
	if err != nil {
		return
	}
	f, err := os.Open(flags.Input)
	if err != nil {
		return
	}
	defer f.Close()
	data, err = readInput(f)
	return
}

func readInput(r io.Reader) (data any, err error) {
	err = yaml.NewDecoder(bufio.NewReader(r)).Decode(&data)
	return
}

func readPackageAuto(input string) (string, error) {
	if input == "(stdin)" {
		dir, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return readPackageName(dir)
	}
	return readPackageName(filepath.Dir(input))
}

func readPackageName(dir string) (string, error) {
	list, err := stacktrace.Trace2(os.ReadDir(dir))
	if err != nil {
		return "", err
	}
	for _, v := range list {
		if !v.IsDir() && strings.HasSuffix(v.Name(), ".go") {
			if n, err := readPackage(v.Name()); err == nil { // if NO error
				return n, nil
			}
		}
	}
	return reReplace.ReplaceAllString(filepath.Base(dir), "_"), nil
}

var reReplace = regexp.MustCompile(`[^a-zA-Z0-9_]`)

func readPackage(file string) (string, error) {
	f, err := stacktrace.Trace2(os.Open(file))
	if err != nil {
		return "", err
	}
	defer f.Close()
	s := bufioscanner.New(bufio.NewScanner(f))
	for _, line := range s.Text() {
		line = strings.TrimSpace(line)
		m := rePackage.FindStringSubmatch(line)
		if len(m) >= 2 {
			return m[1], nil
		}
	}
	return "", errNotFound
}

var rePackage = regexp.MustCompile(`^package\s+([a-zA-Z][a-zA-Z0-9_]*)`)

var errNotFound = errors.New("not found")
