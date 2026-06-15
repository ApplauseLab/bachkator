## Image Targets

Image targets synthesize OCI build commands:

```hcl
resource "base_image" {}

image "base" {
  builder    = "container"
  image      = "example/base"
  tags       = ["local"]
  dockerfile = "Dockerfile.base"
  context    = "."
  platform   = "linux/amd64"
  produces   = [resource.base_image]
}

image "app" {
  builder    = "container"
  image      = "example/app"
  tags       = ["latest"]
  dockerfile = "Dockerfile"
  push       = true
  build_args = {
    BASE_IMAGE = image.base.tag
  }
  inputs = [resource.base_image, "Dockerfile", "cmd"]
}
```

Fields:

- `builder`: build executable. Defaults to `OCI_BUILDER`, then `docker`.
- `image`: image name. Defaults to the image target name without `image/`.
- `tags`: tags to apply.
- `dockerfile`: Dockerfile path. Defaults to `Dockerfile`.
- `context`: build context. Defaults to `.`.
- `platform`: optional platform.
- `push`: push every resolved tag after a successful build. Docker-compatible builders run `<builder> push <tag>`; Apple `container` runs `container image push <tag>`.
- `build_args`: map of build args.
- `build_args_list`: list of already-rendered `KEY=value` build args.
- `lock`: optional in-run named lock. Useful for builders such as Apple `container` that share host resources.
- `inputs`, `depends_on`, `produces`: same behavior as shell targets.

`image.name.tag` resolves to the first full image tag for use in build args.
