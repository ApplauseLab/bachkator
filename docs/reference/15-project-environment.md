## Project Environment

Top-level `env` blocks define project-wide operation environment. Entries are stored and fingerprinted in sorted key order.

```hcl
var bla {}

var "foo" {
  default = "foo"
}

var foobar {
  default = "${var.foo}bar"
}

env {
  ENV_1 = "b"
  ENV_2 = "${ENV_1} b ${var.bla}"
}
```

Environment values can reference variables with `var.name` and earlier resolvable environment keys directly by name. HCL string literals use double quotes.
