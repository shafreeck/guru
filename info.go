package main

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

type GuruInfo struct {
	g     *Guru
	opts  *ChatCommandOptions
	copts *ChatOptions
}

func NewGuruInfo(g *Guru, opts *ChatCommandOptions) *GuruInfo {
	return &GuruInfo{g: g, opts: opts}
}
func (g *GuruInfo) infoCommand() string {
	dict := buildFieldIndex(g.opts)
	// only copts fields could be set
	coptsDict := buildFieldIndex(g.copts)

	var settable []string
	var unsettable []string
	for k, v := range dict {
		if _, ok := coptsDict[k]; ok {
			settable = append(settable, fmt.Sprint(fmt.Sprintf("%-30s", k),
				g.g.textStyle.Render(fmt.Sprint(v))))
			continue
		}

		if k == "openai-api-key" {
			unsettable = append(unsettable, fmt.Sprint(fmt.Sprintf("%-30s", k),
				g.g.errStyle.Render("sk-"+strings.Repeat("*", 48))))
			continue
		}
		unsettable = append(unsettable, fmt.Sprint(fmt.Sprintf("%-30s", k),
			g.g.errStyle.Render(fmt.Sprint(v))))
	}
	sort.StringSlice(unsettable).Sort()
	sort.StringSlice(settable).Sort()
	for _, line := range unsettable {
		fmt.Fprintln(g.g.stdout, line)
	}
	fmt.Fprintln(g.g.stdout, strings.Repeat("-", 30))
	for _, line := range settable {
		fmt.Fprintln(g.g.stdout, line)
	}
	return ""
}
func (g *GuruInfo) setCommand() (_ string) {
	opts := struct {
		Key   string `cortana:"key, -,"`
		Value string `cortana:"val, -,"`
	}{}
	if usage := builtins.Parse(&opts); usage {
		g.g.Print(builtins.UsageString())
		return
	}
	if opts.Key == "" {
		g.g.Errorln("key is required")
		return
	}

	settable := buildFieldIndex(g.copts)
	dict := buildFieldIndex(g.opts)

	rv, ok := dict[opts.Key]
	if !ok {
		g.g.Errorln("key is not found: ", opts.Key)
		return
	}
	if _, ok := settable[opts.Key]; !ok {
		g.g.Errorln("key is not settable in interactive mode, use config instead: ", opts.Key)
		return
	}

	for rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.String:
		rv.SetString(opts.Value)
	case reflect.Bool:
		if opts.Value == "" {
			rv.SetBool(true)
			return
		}
		b, err := strconv.ParseBool(opts.Value)
		if err != nil {
			g.g.Errorln(err)
			return
		}
		rv.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32,
		reflect.Int64:
		i, err := strconv.ParseInt(opts.Value, 10, 64)
		if err != nil {
			d, err := time.ParseDuration(opts.Value)
			if err != nil {
				g.g.Errorln(err)
				return
			}
			rv.SetInt(int64(d))
			return
		}
		rv.SetInt(i)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32,
		reflect.Uint64:
		u, err := strconv.ParseUint(opts.Value, 10, 64)
		if err != nil {
			g.g.Errorln(err)
			return
		}
		rv.SetUint(u)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(opts.Value, 64)
		if err != nil {
			g.g.Errorln(err)
			return
		}
		rv.SetFloat(f)
	default:
		g.g.Errorln(fmt.Errorf("unsupported kind: %s", rv.Kind().String()))
		return
	}
	return ""
}

func buildFieldIndex(v interface{}) map[string]reflect.Value {
	m := make(map[string]reflect.Value)
	analyze("", reflect.ValueOf(v), m)
	return m
}

func analyze(prefix string, val reflect.Value, m map[string]reflect.Value) {
	for val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return
	}
	for i := 0; i < val.NumField(); i++ {
		vField := val.Field(i)
		tField := val.Type().Field(i)

		tag := strings.Split(tField.Tag.Get("yaml"), ",")[0]
		if tag == "-" {
			continue
		}
		if tag == "" {
			tag = tField.Name
		}

		if vField.Kind() == reflect.Struct {
			analyze(fmt.Sprintf("%s%s.", prefix, tag), vField, m)
			continue
		}

		m[fmt.Sprintf("%s%s", prefix, tag)] = vField
	}
}

func (g *GuruInfo) registerBuiltinCommands() {
	builtins.AddCommand(":info", g.infoCommand, "show guru options")
	builtins.AddCommand(":set", g.setCommand, "set guru options")
}
