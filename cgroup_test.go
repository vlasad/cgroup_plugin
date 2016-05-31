// +build linux

package cgroup

import (
	"testing"

	"github.com/influxdata/telegraf/testutil"
	"github.com/stretchr/testify/require"
)

var cg1 = &CGroup{
	Rules: []Rule{
		Rule{
			Prefix: "testdata/memory",
			Paths:  []string{"/"},
			Fields: []string{"memory.empty", "memory.max_usage_in_bytes", "memory.limit_in_bytes", "memory.stat", "memory.use_hierarchy"},
		},
		Rule{
			Prefix: "testdata/cpu",
			Paths:  []string{"/"},
			Fields: []string{"cpuacct.usage_percpu"},
		},
	},
}

func TestCgroupStatistics_1(t *testing.T) {
	var acc testutil.Accumulator

	err := cg1.Gather(&acc)
	require.NoError(t, err)

	tags := map[string]string{
		"path": "testdata/memory/memory.stat",
	}
	fields := map[string]interface{}{
		"cache":       1739362304123123123,
		"rss":         1775325184,
		"rss_huge":    778043392,
		"mapped_file": 421036032,
		"dirty":       -307200,
	}
	acc.AssertContainsTaggedFields(t, "cgroup:memory.stat", fields, tags)

	tags = map[string]string{
		"path": "testdata/cpu/cpuacct.usage_percpu",
	}
	fields = map[string]interface{}{
		"value_0": -1452543795404,
		"value_1": 1376681271659,
		"value_2": 1450950799997,
		"value_3": -1473113374257,
	}
	acc.AssertContainsTaggedFields(t, "cgroup:cpuacct.usage_percpu", fields, tags)

	tags = map[string]string{
		"path": "testdata/memory/memory.max_usage_in_bytes",
	}
	fields = map[string]interface{}{
		"value_0": 0,
		"value_1": -1,
		"value_2": 2,
	}
	acc.AssertContainsTaggedFields(t, "cgroup:memory.max_usage_in_bytes", fields, tags)

	tags = map[string]string{
		"path": "testdata/memory",
	}
	fields = map[string]interface{}{
		"memory.limit_in_bytes": 223372036854771712,
		"memory.use_hierarchy":  "12-781",
	}
	acc.AssertContainsTaggedFields(t, "cgroup:memory", fields, tags)
}

var cg2 = &CGroup{
	Rules: []Rule{
		Rule{
			Prefix: "testdata/memory",
			Paths:  []string{"*"},
			Fields: []string{"memory.limit_in_bytes"},
		},
	},
}

func TestCgroupStatistics_2(t *testing.T) {
	var acc testutil.Accumulator

	err := cg2.Gather(&acc)
	require.NoError(t, err)

	tags := map[string]string{
		"path": "testdata/memory/group_1",
	}
	fields := map[string]interface{}{
		"memory.limit_in_bytes": 223372036854771712,
	}
	acc.AssertContainsTaggedFields(t, "cgroup:memory", fields, tags)

	tags = map[string]string{
		"path": "testdata/memory/group_2",
	}
	acc.AssertContainsTaggedFields(t, "cgroup:memory", fields, tags)
}

var cg3 = &CGroup{
	Rules: []Rule{
		Rule{
			Prefix: "testdata/memory",
			Paths:  []string{"*/*", "group_2"},
			Fields: []string{"memory.limit_in_bytes"},
		},
	},
}

func TestCgroupStatistics_3(t *testing.T) {
	var acc testutil.Accumulator

	err := cg3.Gather(&acc)
	require.NoError(t, err)

	tags := map[string]string{
		"path": "testdata/memory/group_1/group_1_1",
	}
	fields := map[string]interface{}{
		"memory.limit_in_bytes": 223372036854771712,
	}
	acc.AssertContainsTaggedFields(t, "cgroup:memory", fields, tags)

	tags = map[string]string{
		"path": "testdata/memory/group_1/group_1_2",
	}
	acc.AssertContainsTaggedFields(t, "cgroup:memory", fields, tags)

	tags = map[string]string{
		"path": "testdata/memory/group_2",
	}
	acc.AssertContainsTaggedFields(t, "cgroup:memory", fields, tags)
}
