# Fakalelei

*The tool that fixes what others got wrong.*

Fakalelei (Tongan: "to fix, reconcile, make amends"; pronounced *fah-kah-leh-LAY*) is an experimental infrastructure-as-code platform designed to reconcile the gaps left by existing IaC tools. It approaches infrastructure through a **reconciliation-first**, **dependency-aware**, and **cloud-agnostic** lens—enabling declarative, deterministic authoring without sacrificing the flexibility to model complex, multi-provider environments.

## Key Principles

- **Reconciliation-first**: Continuous control-loop designs over one-shot apply-only flows
- **Dependency-DAG planning**: Preserve graph-based ordering and cycle safety
- **Provider-agnostic**: Cloud-neutral core with schema-driven provider integration
- **Declarative intent**: Conditionals, loops, and transformations while staying deterministic
- **Explicit lifecycle**: Model create/read/update/delete and custom actions clearly

## Learn More

See `docs/` for architecture, design decisions, and detailed specifications.
