// Command dalibre-plugin is for writing Dalibre plugins: it makes a new
// one, runs it as Dalibre would without the app, packs it, and installs
// packages into the plugins folder.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"

	"dalibre/internal/plugin"
	"dalibre/internal/settings"
	"dalibre/sdk"
)

const usage = `dalibre-plugin is for writing Dalibre plugins.

  dalibre-plugin init <dir> [--id ID] [--name NAME]   start a plugin from the template
  dalibre-plugin check <dir>                          validate plugin.json and load main.js
  dalibre-plugin search <dir> <text> [--page N] [--cursor C] [--json]
  dalibre-plugin download <dir> <text> [--item N] [--format EPUB] [-o DIR]
  dalibre-plugin pack <dir> [-o DIR]                  make <id>-<version>.dalibre-plugin
  dalibre-plugin install <package> [--yes]            install into Dalibre's plugins folder
  dalibre-plugin uninstall <id>
  dalibre-plugin list

search and download take --set key=value for the plugin's settings; the
values saved in Dalibre are used otherwise. The plugins folder is
$DALIBRE_PLUGINS, or Dalibre's in your configuration folder.
`

func main() {
	if len(os.Args) < 2 || os.Args[1] == "-h" || os.Args[1] == "--help" || os.Args[1] == "help" {
		fmt.Print(usage)
		return
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	cmds := map[string]func(context.Context, []string) error{
		"init": cmdInit, "check": cmdCheck, "search": cmdSearch, "download": cmdDownload,
		"pack": cmdPack, "install": cmdInstall, "uninstall": cmdUninstall, "list": cmdList,
	}
	cmd, ok := cmds[os.Args[1]]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
	if err := cmd(ctx, os.Args[2:]); err != nil {
		report(err)
		os.Exit(1)
	}
}

// report prints an error, with what a plugin's author needs to fix it.
func report(err error) {
	var pe *plugin.Error
	if errors.As(err, &pe) {
		fmt.Fprintf(os.Stderr, "error (%s): %s\n", pe.Kind, pe.Error())
		if pe.Stack != "" {
			fmt.Fprintf(os.Stderr, "\n%s\n", pe.Stack)
		}
		return
	}
	fmt.Fprintln(os.Stderr, "error:", err)
}

// parse parses flags wherever they are among the arguments, and returns
// the arguments that aren't flags, checking there are want of them.
func parse(fs *flag.FlagSet, args []string, want ...string) []string {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			os.Exit(2)
		}
		if fs.NArg() == 0 {
			break
		}
		pos = append(pos, fs.Arg(0))
		args = fs.Args()[1:]
	}
	if len(pos) != len(want) {
		fmt.Fprintf(os.Stderr, "usage: dalibre-plugin %s %s\n", fs.Name(), "<"+strings.Join(want, "> <")+">")
		os.Exit(2)
	}
	return pos
}

type setFlags map[string]string

func (s setFlags) String() string { return "" }
func (s setFlags) Set(v string) error {
	k, val, ok := strings.Cut(v, "=")
	if !ok {
		return errors.New("want key=value")
	}
	s[k] = val
	return nil
}

// load loads the plugin in dir with the settings saved in Dalibre and
// those given, logging to stderr.
func load(dir string, set setFlags) (*plugin.Plugin, error) {
	m, err := plugin.ReadManifest(dir)
	if err != nil {
		return nil, err
	}
	values := map[string]string{}
	if st := settings.Load(settings.DefaultPath()).Plugins[m.ID]; st != nil {
		for k, v := range st.Values {
			values[k] = v
		}
	}
	for k, v := range set {
		if !slices.ContainsFunc(m.Settings, func(s plugin.Setting) bool { return s.Key == k }) {
			return nil, fmt.Errorf("--set %s: plugin.json has no setting %q", k, k)
		}
		values[k] = v
	}
	return plugin.Load(dir, plugin.Options{Settings: values, Logf: func(f string, a ...any) {
		fmt.Fprintf(os.Stderr, f+"\n", a...)
	}})
}

func cmdInit(_ context.Context, args []string) error {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	id := fs.String("id", "", "the plugin's id (default: the folder's name)")
	name := fs.String("name", "", "the plugin's name (default: the id)")
	dir := parse(fs, args, "dir")[0]
	if *id == "" {
		*id = strings.ToLower(filepath.Base(dir))
	}
	if *name == "" {
		*name = *id
	}
	if entries, _ := os.ReadDir(dir); len(entries) > 0 {
		return fmt.Errorf("%s isn’t empty", dir)
	}
	files := map[string]string{"dalibre.d.ts": "dalibre.d.ts", "plugin.schema.json": "plugin.schema.json"}
	tmpl, err := templateFiles()
	if err != nil {
		return err
	}
	for _, e := range tmpl {
		files[e] = "template/" + e
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for dst, src := range files {
		data, err := sdk.Files.ReadFile(src)
		if err != nil {
			return err
		}
		text := strings.NewReplacer("{{ID}}", *id, "{{NAME}}", *name).Replace(string(data))
		if err := os.WriteFile(filepath.Join(dir, dst), []byte(text), 0o644); err != nil {
			return err
		}
	}
	if _, err := plugin.ReadManifest(dir); err != nil {
		return fmt.Errorf("%w (is %q a valid id? pass --id)", err, *id)
	}
	fmt.Printf("Made %s. Edit plugin.json and main.js, then: dalibre-plugin search %s \"a title\"\n", dir, dir)
	return nil
}

// templateFiles lists the template's files.
func templateFiles() ([]string, error) {
	entries, err := fs.ReadDir(sdk.Files, "template")
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names, err
}

func cmdCheck(_ context.Context, args []string) error {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	dir := parse(fs, args, "dir")[0]
	p, err := plugin.Load(dir, plugin.Options{Logf: func(string, ...any) {}})
	if err != nil {
		return err
	}
	m := p.Manifest
	fmt.Printf("%s %s (%s), plugin API %d\n", m.Name, m.Version, m.ID, m.API)
	fmt.Printf("capabilities: %s\n", strings.Join(m.Capabilities, ", "))
	for _, fn := range []string{"search", "resolve"} {
		if p.Defines(fn) {
			fmt.Printf("exports %s\n", fn)
		}
	}
	fmt.Println("asks to:")
	for _, d := range m.Permissions.Describe() {
		fmt.Println("  " + d)
	}
	if m.Description == "" {
		fmt.Println("warning: no description; users see it before installing")
	}
	if m.Homepage == "" {
		fmt.Println("warning: no homepage")
	}
	fmt.Println("ok")
	return nil
}

func cmdSearch(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("search", flag.ExitOnError)
	page := fs.Int("page", 1, "the page")
	cursor := fs.String("cursor", "", "the cursor a previous page returned")
	asJSON := fs.Bool("json", false, "print the results as JSON")
	set := setFlags{}
	fs.Var(set, "set", "a setting, as key=value (repeatable)")
	pos := parse(fs, args, "dir", "text")
	p, err := load(pos[0], set)
	if err != nil {
		return err
	}
	res, err := p.Search(ctx, plugin.Query{Text: pos[1], Page: *page, Cursor: *cursor})
	if err != nil {
		return err
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}
	printPage(res)
	return nil
}

func printPage(res plugin.Page) {
	for i, r := range res.Items {
		fmt.Printf("%2d. %s", i+1, r.Title)
		if len(r.Authors) > 0 {
			fmt.Printf(" — %s", strings.Join(r.Authors, ", "))
		}
		fmt.Println()
		var formats []string
		for _, f := range r.Formats {
			s := f.Format
			if f.Size > 0 {
				s += fmt.Sprintf(" %.1f MB", float64(f.Size)/(1<<20))
			}
			formats = append(formats, s)
		}
		fmt.Printf("    id %s · %s", r.ID, strings.Join(formats, ", "))
		if r.Language != "" {
			fmt.Printf(" · %s", r.Language)
		}
		fmt.Println()
	}
	if len(res.Items) == 0 {
		fmt.Println("no results")
	}
	if res.HasMore {
		next := "--page N+1"
		if res.Cursor != "" {
			next = "--cursor " + res.Cursor
		}
		fmt.Printf("more: %s\n", next)
	}
	for _, w := range res.Warnings {
		fmt.Fprintln(os.Stderr, "warning: search returned", w)
	}
}

func cmdDownload(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("download", flag.ExitOnError)
	item := fs.Int("item", 1, "which result to download, from 1")
	format := fs.String("format", "", "the format (default: the result's first)")
	out := fs.String("o", ".", "the folder to save in")
	set := setFlags{}
	fs.Var(set, "set", "a setting, as key=value (repeatable)")
	pos := parse(fs, args, "dir", "text")
	p, err := load(pos[0], set)
	if err != nil {
		return err
	}
	res, err := p.Search(ctx, plugin.Query{Text: pos[1]})
	if err != nil {
		return err
	}
	for _, w := range res.Warnings {
		fmt.Fprintln(os.Stderr, "warning: search returned", w)
	}
	if *item < 1 || *item > len(res.Items) {
		return fmt.Errorf("--item %d: search returned %d results", *item, len(res.Items))
	}
	r := res.Items[*item-1]
	if *format == "" {
		*format = r.Formats[0].Format
	}
	req, err := p.Resolve(ctx, r, *format)
	if err != nil {
		return err
	}
	fmt.Printf("%s (%s) from %s\n", r.Title, strings.ToUpper(*format), req.URL)
	path, err := p.Download(ctx, r, *format, *out, func(done, total int64) {
		if total > 0 {
			fmt.Fprintf(os.Stderr, "\r%.1f / %.1f MB", float64(done)/(1<<20), float64(total)/(1<<20))
		} else {
			fmt.Fprintf(os.Stderr, "\r%.1f MB", float64(done)/(1<<20))
		}
	})
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return err
	}
	if _, err := p.Cover(ctx, r); err != nil {
		fmt.Fprintln(os.Stderr, "warning: cover:", err)
	}
	fmt.Println("saved", path)
	return nil
}

func cmdPack(_ context.Context, args []string) error {
	fs := flag.NewFlagSet("pack", flag.ExitOnError)
	out := fs.String("o", ".", "the folder to write the package in")
	dir := parse(fs, args, "dir")[0]
	path, err := plugin.Pack(dir, *out)
	if err != nil {
		return err
	}
	fmt.Println("packed", path)
	return nil
}

func cmdInstall(_ context.Context, args []string) error {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	yes := fs.Bool("yes", false, "grant the permissions without asking")
	archive := parse(fs, args, "package")[0]
	m, err := plugin.Inspect(archive)
	if err != nil {
		return err
	}
	s := settings.Load(settings.DefaultPath())
	st := s.Plugin(m.ID)
	have := plugin.Permissions{}
	if st.Granted != nil {
		have = plugin.Permissions{Hosts: st.Granted.Hosts, LocalNetwork: st.Granted.LocalNetwork}
	}
	if !have.Covers(m.Permissions) {
		fmt.Printf("%s %s", m.Name, m.Version)
		if m.Author != "" {
			fmt.Printf(" by %s", m.Author)
		}
		fmt.Printf(" (%s) asks to:\n", m.ID)
		for _, d := range m.Permissions.Describe() {
			fmt.Println("  " + d)
		}
		if !*yes {
			fmt.Print("Allow? [y/N] ")
			line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
			if a := strings.ToLower(strings.TrimSpace(line)); a != "y" && a != "yes" {
				return errors.New("not installed")
			}
		}
		st.Granted = &settings.Grant{Hosts: m.Permissions.Hosts, LocalNetwork: m.Permissions.LocalNetwork}
	}
	if _, err := plugin.Install(archive, plugin.DefaultDir()); err != nil {
		return err
	}
	if err := s.Save(); err != nil {
		return err
	}
	fmt.Printf("installed %s %s in %s; restart Dalibre to use it\n", m.Name, m.Version, plugin.DefaultDir())
	return nil
}

func cmdUninstall(_ context.Context, args []string) error {
	fs := flag.NewFlagSet("uninstall", flag.ExitOnError)
	id := parse(fs, args, "id")[0]
	if err := plugin.Uninstall(plugin.DefaultDir(), id); err != nil {
		return err
	}
	s := settings.Load(settings.DefaultPath())
	delete(s.Plugins, id)
	return s.Save()
}

func cmdList(_ context.Context, args []string) error {
	parse(flag.NewFlagSet("list", flag.ExitOnError), args)
	root := plugin.DefaultDir()
	s := settings.Load(settings.DefaultPath())
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		fmt.Println("no plugins in", root)
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		m, err := plugin.ReadManifest(filepath.Join(root, e.Name()))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			fmt.Printf("%-24s error: %v\n", e.Name(), err)
			continue
		}
		state := "on"
		st := s.Plugins[m.ID]
		switch {
		case st != nil && st.Disabled:
			state = "off"
		case st == nil || st.Granted == nil ||
			!(plugin.Permissions{Hosts: st.Granted.Hosts, LocalNetwork: st.Granted.LocalNetwork}).Covers(m.Permissions):
			state = "waiting for permission"
		}
		fmt.Printf("%-24s %-10s %s (%s)\n", m.ID, m.Version, m.Name, state)
	}
	return nil
}
