## Embedded Reference

Bachkator embeds the `docs/` reference into the binary.

```sh
bach reference
bach reference project
bach reference plugins
```

`bach reference` lists all headings. `bach reference <query>` prints matching sections. It does not require a `Bachfile`, so agents can use it before project loading succeeds.
