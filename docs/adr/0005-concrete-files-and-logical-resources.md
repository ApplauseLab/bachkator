# Concrete Files and Logical Resources

Bachkator keeps concrete file evidence separate from logical resource evidence. **Inputs** and **Outputs** are paths that can be matched, hashed, or consumed by quality reports, while **Resources** and `produces` represent logical capabilities or produced identities without requiring Bachkator to hash large directories such as dependencies, generated workspaces, or base images. Future cache and affected-target improvements should preserve this split rather than collapsing everything into a generic artifact model.
