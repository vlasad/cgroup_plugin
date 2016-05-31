# CGroup Input Plugin For Telegraf Agent

This input plugin will capture specific statistics per cgroup.

Following file formats are supported:

* Single value

```
VAL\n
```

* New line separated values

```
VAL0\n
VAL1\n
```

* Space separated values

```
VAL0 VAL1 ...\n
```

* New line separated key-space-value's

```
KEY0 VAL0\n
KEY1 VAL1\n
```


All files with single value will be combined, for example:

```
{
  "fields": {
    "memory.limit_in_bytes": 9223372036854771712,
    "memory.max_usage_in_bytes": 0
  },
  "name": "cgroup:memory",
  "tags": {
    "path": "/sys/fs/cgroup/memory"
  },
  "timestamp": 1464183720
}
```


### Tags:

All measurements have the following tags:
  - path


### Configuration:

```
# [[inputs.cgroup]]
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
```
