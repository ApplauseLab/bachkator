# Graph-Load Plugins

Bachkator plugins are graph-load extenders: they run while loading a **Project** and may contribute input sets or dependency/input patches to existing **Targets**. They should remain side-effect-free because commands like `bach list`, `bach explain`, and `bach affected` load the graph and may run plugins. Execution hooks, state-store extensions, and plugin-defined target kinds are separate future extension points, not part of the current plugin contract.
