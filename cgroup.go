// +build linux

package cgroup

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/plugins/inputs"
)

type Rule struct {
	Prefix string
	Paths  []string
	Fields []string
}

type CGroup struct {
	Prefix string
	Rules  []Rule
}

var sampleConfig = `
	## To don't duplicate full path, you can define prefix.
	## This optional prefix is global for all rules.
	# prefix = "/cgroup/"

	## If global prefix is not defined, it is necessary to specify full path to cgroup. For example:
	# [[inputs.cgroup.rules]]
	#   paths = [
	#     "/cgroup/memory",           # root cgroup
	#     "/cgroup/memory/child1",    # container cgroup
	#     "/cgroup/memory/child2/*",  # all children cgroups under child2, but not child2 itself
	#   ]
	#   fields = ["memory.max_usage_in_bytes", "memory.limit_in_bytes"]

	## If prefix is defined, it is necessary to specify only relative path to cgroup. For example:
	# [[inputs.cgroup.rules]]
	#   ## Also It's possible to define prefix per rule. Instead of global prefix, this one will be used for the rule.
	#   prefix = "/cgroup/cpu/"   # optional
	#   paths = [
	#     "/",                    # root cgroup
	#     "child1",               # container cgroup
	#     "*",                    # all container cgroups
	#     "child2/*",             # all children cgroups under child2, but not child2 itself
	#     "*/*",                  # all children cgroups under each container cgroup
	#   ]
	#   fields = ["cpuacct.usage", "cpu.cfs_period_us", "cpu.cfs_quota_us"]
`

func (g *CGroup) SampleConfig() string {
	return sampleConfig
}

func (g *CGroup) Description() string {
	return "Read specific statistics per cgroup"
}

func (g *CGroup) Gather(acc telegraf.Accumulator) error {
	if err := g.normalize(); err != nil {
		return err
	}

	for _, r := range g.Rules {
		if err := r.gather(acc); err != nil {
			return err
		}
	}
	return nil
}

func (g *CGroup) normalize() error {
	g.Prefix = strings.TrimSpace(g.Prefix)

	for i, r := range g.Rules {
		g.Rules[i].Prefix = strings.TrimSpace(g.Rules[i].Prefix)
		if g.Rules[i].Prefix == "" {
			g.Rules[i].Prefix = g.Prefix
		}

		pathsCount := 0
		for j := range r.Paths {
			if strings.TrimSpace(r.Paths[j]) != "" {
				pathsCount++
			}
		}
		if pathsCount == 0 {
			return fmt.Errorf("rule #%d has not any path", i)
		}

		fieldsCount := 0
		for j := range r.Fields {
			if strings.TrimSpace(r.Fields[j]) != "" {
				fieldsCount++
			}
		}
		if fieldsCount == 0 {
			return fmt.Errorf("rule #%d has not any field", i)
		}
	}

	return nil
}

func (r *Rule) gather(acc telegraf.Accumulator) error {
	list := make(chan dirInfo)
	go r.generateDirs(list)

	for dir := range list {
		if dir.err != nil {
			return dir.err
		}
		if err := r.gatherDir(dir.path, acc); err != nil {
			return err
		}
	}

	return nil
}

type dirInfo struct {
	path string
	err  error
}

func isDir(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return fileInfo.IsDir(), nil
}

func (r *Rule) generateDirs(list chan<- dirInfo) {
	for _, dir := range r.Paths {
		dir = path.Clean(path.Join(r.Prefix, dir))

		items, err := filepath.Glob(dir)
		if err != nil {
			list <- dirInfo{err: err}
			return
		}

		for _, item := range items {
			ok, err := isDir(item)
			if err != nil {
				list <- dirInfo{err: err}
				return
			}
			if ok {
				list <- dirInfo{path: item}
			}
		}
	}
	close(list)
}

func (r *Rule) measurement() string {
	return strings.Split(r.Fields[0], ".")[0]
}

func (r *Rule) gatherDir(dir string, acc telegraf.Accumulator) error {
	singleValues := make(map[string]interface{})

	for _, file := range r.Fields {
		filePath := path.Join(dir, file)
		raw, err := ioutil.ReadFile(filePath)
		if err != nil {
			return err
		}
		if len(raw) == 0 {
			continue
		}

		fi := fileInfo{data: raw, path: filePath}
		fields, tags, err := fi.parse()
		if err != nil {
			return err
		}

		if len(fields) == 1 {
			singleValues[file] = fields["value"]
		} else {
			tags["path"] = filePath
			acc.AddFields("cgroup:"+file, fields, tags)
		}
	}

	acc.AddFields("cgroup:"+r.measurement(), singleValues, map[string]string{"path": dir})

	return nil
}

// ======================================================================

type fileInfo struct {
	data []byte
	path string
}

func (d *fileInfo) format() (*fileFormat, error) {
	for _, f := range fileFormats {
		ok, err := f.match(d.data)
		if err != nil {
			return nil, err
		}
		if ok {
			return &f, nil
		}
	}

	return nil, fmt.Errorf("%v: unknown file format", d.path)
}

func (d *fileInfo) parse() (map[string]interface{}, map[string]string, error) {
	format, err := d.format()
	if err != nil {
		return nil, nil, err
	}

	f, t := format.parser(d.data)
	return f, t, nil
}

// ======================================================================

type fileFormat struct {
	name    string
	pattern string
	parser  func(b []byte) (map[string]interface{}, map[string]string)
}

const keyPattern = "[[:alpha:]_]+"
const valuePattern = "[\\d-]+"

var fileFormats = [...]fileFormat{
	// 	VAL\n
	fileFormat{
		name:    "Single value",
		pattern: "^" + valuePattern + "\n$",
		parser: createParser(
			"^("+valuePattern+")\n$",
			func(matches [][]string, fields map[string]interface{}, tags map[string]string) {
				fields["value"] = numberOrString(matches[0][1])
			},
		),
	},
	// 	VAL0\n
	// 	VAL1\n
	// 	...
	fileFormat{
		name:    "New line separated values",
		pattern: "^(" + valuePattern + "\n){2,}$",
		parser: createParser(
			"("+valuePattern+")\n",
			func(matches [][]string, fields map[string]interface{}, tags map[string]string) {
				for i, v := range matches {
					fields["value_"+strconv.Itoa(i)] = numberOrString(v[1])
				}
			},
		),
	},
	// 	VAL0 VAL1 ...\n
	fileFormat{
		name:    "Space separated values",
		pattern: "^(" + valuePattern + " )+\n$",
		parser: createParser(
			"("+valuePattern+") ",
			func(matches [][]string, fields map[string]interface{}, tags map[string]string) {
				for i, v := range matches {
					fields["value_"+strconv.Itoa(i)] = numberOrString(v[1])
				}
			},
		),
	},
	// 	KEY0 VAL0\n
	// 	KEY1 VAL1\n
	// 	...
	fileFormat{
		name:    "New line separated key-space-value's",
		pattern: "^(" + keyPattern + " " + valuePattern + "\n)+$",
		parser: createParser(
			"("+keyPattern+") ("+valuePattern+")\n",
			func(matches [][]string, fields map[string]interface{}, tags map[string]string) {
				for _, v := range matches {
					fields[v[1]] = numberOrString(v[2])
				}
			},
		),
	},
}

func numberOrString(s string) interface{} {
	i, err := strconv.Atoi(s)
	if err == nil {
		return i
	}

	return s
}

func (f fileFormat) match(b []byte) (bool, error) {
	ok, err := regexp.Match(f.pattern, b)
	if err != nil {
		return false, err
	}
	if ok {
		return true, nil
	}
	return false, nil
}

func createParser(
	itemPattern string,
	fn func(m [][]string, f map[string]interface{}, t map[string]string),
) func(b []byte) (map[string]interface{}, map[string]string) {
	return func(b []byte) (map[string]interface{}, map[string]string) {
		re := regexp.MustCompile(itemPattern)
		matches := re.FindAllStringSubmatch(string(b), -1)
		fields := make(map[string]interface{})
		tags := make(map[string]string)

		fn(matches, fields, tags)

		return fields, tags
	}
}

func init() {
	inputs.Add("cgroup", func() telegraf.Input { return &CGroup{} })
}
