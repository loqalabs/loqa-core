# Skill Examples

Sample manifests live under `skills/examples`. Validate a manifest with:

```bash
go run ./cmd/loqa-skill validate --file skills/examples/timer/skill.yaml
```

The validation command ensures required metadata, runtime mode, and permissions are present before loading a skill into Loqa.
