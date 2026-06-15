## Environment Profiles

Profiles are named environment overlays for staging variants and operator-specific settings:

```hcl
profile "staging" {
  env {
    NAMESPACE   = "atelier-staging"
    AWS_PROFILE = "atelier-staging"
    PUBLIC_HOST = "staging.example.com"
  }
}

profile "staging-kristiyan" {
  env {
    NAMESPACE = "atelier-staging-kristiyan"
  }
}
```

Select profiles with `--profile`. The flag may be repeated, and later profiles override earlier profile values:

```sh
bach --profile staging --profile staging-kristiyan run shell/render
```

Selected profile values overlay after top-level `env` and before target `env`. Unknown selected profiles are errors. Selected profile names and resolved profile values are included in target fingerprints.
