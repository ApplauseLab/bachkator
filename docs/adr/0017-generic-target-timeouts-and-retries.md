# Generic Target Timeouts and Retries

Bachkator should add timeout and retry policy to the generic target runtime facet rather than to a specific target kind. Timeout should use duration strings such as `timeout = "5m"`; retry should use a block such as `retry { attempts = 3 backoff = "2s" }`. Retry applies to command execution and completion-contract failures, not preflight failures or quality-gate failures; pipeline timeout may wrap the whole pipeline, but whole-pipeline retry should not be enabled by default.
