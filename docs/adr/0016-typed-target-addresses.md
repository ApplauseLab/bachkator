# Typed Target Addresses

Bachkator should replace slash target identities such as `shell/test` with typed **Target Addresses** such as `shell.test`, `pipeline.release`, and `image.app`. HCL references may use expression references such as `depends_on = [shell.test]` or string dot references such as `"shell.test"`, while slash references should be rejected with migration guidance. The CLI may resolve unqualified names such as `test` only when exactly one target has that name across target kinds.
