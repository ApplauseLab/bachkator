## Resources

Resources model produced capabilities or artifacts without hashing large output trees:

```hcl
resource "workspace_deps" {}

shell "install" {
  command  = ["bun", "install"]
  produces = [resource.workspace_deps]
  outputs  = ["node_modules"]
}

shell "test" {
  command = ["bun", "test"]
  inputs  = [resource.workspace_deps]
}
```

When a target consumes a produced resource, Bachkator automatically adds an implicit dependency on the producer.
